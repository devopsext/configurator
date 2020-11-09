/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	//appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	samplev1alpha1 "github.com/devopsext/configurator/pkg/apis/rtcfg/v1alpha1"
	clientset "github.com/devopsext/configurator/pkg/generated/clientset/versioned"
	samplescheme "github.com/devopsext/configurator/pkg/generated/clientset/versioned/scheme"
	informers "github.com/devopsext/configurator/pkg/generated/informers/externalversions/rtcfg/v1alpha1"
	listers "github.com/devopsext/configurator/pkg/generated/listers/rtcfg/v1alpha1"
	ps "github.com/shirou/gopsutil/v3/process"
)

const controllerAgentName = "runtime-configuration"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Generic is synced
	SuccessSynced = "Synced"

	// Skipped is used as part of the Event 'reason' when a Generic is already processed
	Skipped = "Skipped"

	// ErrResourceExists is used as part of the Event 'reason' when a Generic fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Generic"
	// MessageResourceSynced is the message used for an Event fired when a Generic
	// is synced successfully
	MessageResourceSynced = "Generic synced successfully"

	MessageResourceSkipped = "Skipped, already processed"
)

// Controller is the controller implementation for Generic resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// rtCfgclientset is a clientset for our own API group rtcfg.dvext.io
	rtCfgclientset clientset.Interface

	genericsLister listers.GenericLister
	genericsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// intlCache is a controller cache to track latest version of successfully processed
	// generic object
	intlCache map[string]*samplev1alpha1.Generic

	// config is a controller config that holds parameters to operate against the service
	// for which runtime configuration is changed
	config []ctrlConfig

	// selfLabels is a map of pod's labels where the controller runs
	selfLabels map[string]string

	// selfNamespace is a pod's namespaces where the controller runs
	selfNamespace string
}

type ctrlConfig struct {
	PidFinder struct {
		SvcPattern string `yaml:"service_pattern"`
		//Internal storage
		svcPatternRegexp *regexp.Regexp
		svcProc          *ps.Process
		svcName          string

		ParentPattern string `yaml:"parent_pattern"`
		//Internal storage
		parentPatternRegexp *regexp.Regexp
		parentProc          *ps.Process
		parentName          string

		SupervisedSvcName string `yaml:"supervised_service_name"`
	} `yaml:"pid_finder"`

	Config struct {
		StdConfigTemplate string            `yaml:"standard_config"`
		Parameters        map[string]string `yaml:"parameters"`
	} `yaml:"config"`
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	genericInformer informers.GenericInformer,
	configFile string,
	selfNamespace string,
	selfPodName string) (*Controller, error) {

	klog.V(4).Info("Parsing config %q", configFile)
	parsedConfigs, err := parseConfig(configFile)
	if err != nil {
		return nil, err
	}
	//Validating & preparing cfg:
	cfgs := parsedConfigs[:0]
	for _, cfg := range parsedConfigs {
		cfg.PidFinder.parentPatternRegexp, err = regexp.Compile(cfg.PidFinder.ParentPattern)
		if err != nil {
			return nil, fmt.Errorf("Can't compile regex %q: %v", cfg.PidFinder.ParentPattern, err)
		}

		cfg.PidFinder.svcPatternRegexp, err = regexp.Compile(cfg.PidFinder.SvcPattern)
		if err != nil {
			return nil, fmt.Errorf("Can't compile regex %q: %v", cfg.PidFinder.SvcPattern, err)
		}
		cfgs = append(cfgs, cfg)
	}

	// Create event broadcaster
	// Add Generic types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	if err := samplescheme.AddToScheme(scheme.Scheme); err != nil {
		return nil, fmt.Errorf("Can't add Generic types to the default Kubernetes Scheme: %v", err)
	}

	klog.V(4).Info("Creating event broadcaster...")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	//Get self pod labels
	pod, err := kubeclientset.CoreV1().Pods(selfNamespace).Get(context.TODO(), selfPodName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Can't get self pod %s/%s: %v", selfNamespace, selfPodName, err)
	}

	controller := &Controller{
		kubeclientset:  kubeclientset,
		rtCfgclientset: sampleclientset,
		genericsLister: genericInformer.Lister(),
		genericsSynced: genericInformer.Informer().HasSynced,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Generics"),
		recorder:       recorder,
		intlCache:      make(map[string]*samplev1alpha1.Generic),
		config:         cfgs,
		selfLabels:     pod.Labels,
		selfNamespace:  selfNamespace,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Generic resources change
	genericInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueGeneric,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueGeneric(new)
		},
		DeleteFunc: controller.enqueueDelete,
	})

	return controller, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Generic controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.genericsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers...")
	// Launch two workers to process Generic resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	klog.Info("Workers started...")

	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Generic resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			if errors.IsConflict(err) { //we have stale cache
				klog.Info("CONFLICT!")
			}
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		//klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// needToSync will check against local cache if we need to process the generic item
func (c *Controller) needToSync(key string, generic *samplev1alpha1.Generic) (bool, string) {
	var matchedLabels int

	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("Invalid cache key %q: %v", key, err)
		return false, err.Error()
	}

	if namespace != c.selfNamespace {
		return false, fmt.Sprintf("mismatched namespace: generic namespace - %q, self namespace - %q", namespace, c.selfNamespace)
	}

	//checking selectors
	if len(c.selfLabels) == 0 { //No labels specified, we accept any selector in generic
		matchedLabels = len(generic.Selector.MatchLabels)
	} else {

		for k, v := range generic.Selector.MatchLabels {
			if c.selfLabels[k] == v {
				matchedLabels++
			}
		}

		if matchedLabels != len(generic.Selector.MatchLabels) {
			return false, fmt.Sprintf("mismatched labels in pod: %v and\n generic selector: %v", c.selfLabels, generic.Selector.MatchLabels)
		}
	}

	// checking cache
	if _, ok := c.intlCache[key]; !ok || c.intlCache[key].Generation != generic.Generation {
		return true, ""
	}

	return false, fmt.Sprintf("already processed (generation: %d)",generic.Generation)
}

func (c *Controller) injectConfig(generic *samplev1alpha1.Generic, genericDeleted bool) error {
	var err error
	var commands = make([][]string,0,len(generic.Spec.Config.Parameters)*2)

	for index, cfg := range c.config {
		//Skipping cfg element if it is not match to generic.Spec.Service
		if cfg.PidFinder.SupervisedSvcName != generic.Spec.Service {
			return fmt.Errorf("Skipping configuration element %v,\n"+
				"reason: service name mismatch in generic (%s) and configuration element (%s)",
				cfg, generic.Spec.Service, cfg.PidFinder.SupervisedSvcName)
			//continue
		}

		c.config[index], err = getProcess(cfg)
		if err != nil {
			return fmt.Errorf("can't inject config for element %v: %v", cfg, err)
			//continue

		}
		//Injecting config
		klog.Infof("Start injecting configuration %v", cfg)

		for k, v := range generic.Spec.Config.Parameters {
			interpreterCmd, err := parseCommand(k, v, cmdInterpreter, cfg.Config.Parameters, genericDeleted)

			injectCmd, err := parseCommand(k, v, cmdAuto, cfg.Config.Parameters, genericDeleted)
			if err != nil {
				return fmt.Errorf("can't parse command using parameter %s:%s\nReason: %v", k, v, err)
				//continue
			}

			reloadCmd, err := parseCommand(k, v, cmdReload, cfg.Config.Parameters, genericDeleted)
			if err != nil {
				return fmt.Errorf("can't parse reload command using parameter %s:%s\nReason: %v", k, v, err)
				//continue
			}

			commands = append(commands,[]string{interpreterCmd,injectCmd},[]string{interpreterCmd,reloadCmd})
		}

	}

	//Applying changes
	for  _,cmd := range commands {

		if len(cmd) < 2 {
			klog.Errorf("Error parsing command %s, reason: cmd should be 2 item slice, missed interpreter?", cmd)
			continue
		}

		fullCmd := strings.Split(cmd[0]," ") //this is interpreter
		fullCmd = append(fullCmd, cmd[1]) // this is the rest including command itself

		klog.V(2).Infof("Executing command %q ", fullCmd)

		if out, err := exec.Command(fullCmd[0], fullCmd[1:]...).CombinedOutput(); err != nil {
			klog.Errorf("Error executing command:\n%s:\nOutput: %s\nReason: %v", fullCmd, out, err)
			break
		}
	}

	return nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Generic resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	var isDeleted bool

	if isDeleted = strings.HasSuffix(key, ":deleted"); isDeleted {
		key = strings.TrimSuffix(key, ":deleted")
	}

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Generic resource with this namespace/name
	generic, err := c.genericsLister.Generics(namespace).Get(name)
	if err != nil && errors.IsNotFound(err) {
		//Checking internal cache
		ok := false
		generic, ok = c.intlCache[key]
		if !ok {
			// The Generic resource may no longer exist, in which case we stop
			// processing.
			utilruntime.HandleError(fmt.Errorf("generic object '%s' no longer exists in work queue and internal cache ", key))
			return nil
		}
	}else if err != nil {
		return err
	}


	//Here we got generic object.
	//In case of deletion: we got the latest version from cache
	if need, reason := c.needToSync(key, generic); !need && !isDeleted {
		klog.Infof("Skipping object %s, reason: %s", key, reason)
		return nil
	}

	klog.Infof("Processing object %s (generation: %d), is deleted ?: %t", key,generic.Generation, isDeleted)

	//Perform runtime configuration injection
	err = c.injectConfig(generic, isDeleted)
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Generic resource to reflect the
	// current state of the world
	//err = c.updateGenericStatus(generic)
	//if err != nil {
	//	return err
	//}

	//Update internal cache with the version of object that is just processed:
	if !isDeleted {
		c.intlCache[key] = generic.DeepCopy()
		c.recorder.Event(generic, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	} else { //Remove deleted object from cache
		delete(c.intlCache,key)
	}

	return nil
}

func (c *Controller) updateGenericStatus(generic *samplev1alpha1.Generic) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	genericCopy := generic.DeepCopy()
	genericCopy.Status.SeenBy = 1 //We just increment how many controllers seen this

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Generic resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	klog.Infof("Syncing: rv: %s, spec.name: %s, status.avr: %d", genericCopy.ResourceVersion, genericCopy.Name, genericCopy.Status.SeenBy)
	//_, err := c.rtCfgclientset.RtcfgV1alpha1().Generics(generic.Namespace).Update(context.TODO(), genericCopy, metav1.UpdateOptions{})
	_, err := c.rtCfgclientset.RtcfgV1alpha1().Generics(generic.Namespace).UpdateStatus(context.TODO(), genericCopy, metav1.UpdateOptions{})
	return err
}

// enqueueGeneric takes a Generic resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Generic.
func (c *Controller) enqueueGeneric(obj interface{}) {
	var key string
	var err error

	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) enqueueDelete(obj interface{}) {
	var key string
	var err error

	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key + ":deleted")
}
