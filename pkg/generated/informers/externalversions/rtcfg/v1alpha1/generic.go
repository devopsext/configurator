/*
Copyright The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	rtcfgv1alpha1 "github.com/devopsext/configurator/pkg/apis/rtcfg/v1alpha1"
	versioned "github.com/devopsext/configurator/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/devopsext/configurator/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/devopsext/configurator/pkg/generated/listers/rtcfg/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// GenericInformer provides access to a shared informer and lister for
// Generics.
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.GenericLister
}

type genericInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewGenericInformer constructs a new informer for Generic type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewGenericInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredGenericInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredGenericInformer constructs a new informer for Generic type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredGenericInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RtcfgV1alpha1().Generics(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RtcfgV1alpha1().Generics(namespace).Watch(context.TODO(), options)
			},
		},
		&rtcfgv1alpha1.Generic{},
		resyncPeriod,
		indexers,
	)
}

func (f *genericInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredGenericInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&rtcfgv1alpha1.Generic{}, f.defaultInformer)
}

func (f *genericInformer) Lister() v1alpha1.GenericLister {
	return v1alpha1.NewGenericLister(f.Informer().GetIndexer())
}
