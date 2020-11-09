package main

import (
	"fmt"
	"io/ioutil"
	"regexp"

	ps "github.com/shirou/gopsutil/v3/process"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	cmdAuto    = "auto"
	cmdEnable  = "enableCommand"
	cmdDisable = "disableCommand"
	cmdReload  = "reloadCommand"
	cmdInterpreter = "interpreter"
)

func parseConfig(configFile string) ([]ctrlConfig, error) {
	cfgs := make([]ctrlConfig, 10)
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("Can't read configuration file %q: %v", configFile, err)
	}
	if err := yaml.Unmarshal(data, &cfgs); err != nil {
		return nil, fmt.Errorf("Can't parse configuration file %q: %v", configFile, err)
	}

	return cfgs, nil
}

func processExist(process *ps.Process, processName string) (bool, error) {

	if process == nil {
		return false, nil
	}

	exist, err := process.IsRunning()
	if err != nil {
		return false, fmt.Errorf("Can't get process status (pid - %d, name: %s)", process.Pid, processName)
	}

	name, err := process.Name()
	if err != nil {
		return false, fmt.Errorf("Can't get process info (pid - %d, name: %s)", process.Pid, processName)
	}

	return exist && name == processName, nil
}

func findProcess(procs []*ps.Process, namePattern *regexp.Regexp, parent *ps.Process) (*ps.Process, string, int32, error) {
	for _, proc := range procs {
		name, err := proc.Name()
		if err != nil {
			return nil, "", 0, fmt.Errorf("Can't get process name, PID - %d: %v", proc.Pid, err)
		}

		if parent != nil {
			parentPid, err := proc.Ppid()
			if err != nil {
				return nil, "", 0, fmt.Errorf("Can't getting parent process PID: %v", err)
			}
			if namePattern.MatchString(name) && parent.Pid == parentPid {
				return proc, name, parentPid, nil
			}
		}

		if namePattern.MatchString(name) {
			return proc, name, 0, nil
		}

	}
	return nil, "", 0, nil
}

func getProcess(cfg ctrlConfig) (ctrlConfig, error) {
	var svcExist, parentExist bool
	var err error
	var parentPid, servicePid int32
	var procs []*ps.Process

	//Find supervised process
	svcExist, err = processExist(cfg.PidFinder.svcProc, cfg.PidFinder.svcName)
	if err != nil {
		return cfg, fmt.Errorf("Can't check process exsitence %v: %v", cfg.PidFinder, err)
	}

	if !svcExist { //Need to find the process
		procs, err = ps.Processes()
		if err != nil {
			return cfg, fmt.Errorf("can't enumerate running processes: %v", err)
		}

		if cfg.PidFinder.ParentPattern != "" { //then first need to find parent

			parentExist, err = processExist(cfg.PidFinder.parentProc, cfg.PidFinder.parentName)
			if err != nil {
				return ctrlConfig{}, fmt.Errorf("Can't check parent process exsitence %v: %v", cfg.PidFinder, err)
			}
			if !parentExist {

				cfg.PidFinder.parentProc, cfg.PidFinder.parentName, parentPid, err = findProcess(procs, cfg.PidFinder.parentPatternRegexp, nil)
				if err != nil {
					return ctrlConfig{}, fmt.Errorf("Can't find parent process %v: %v", cfg.PidFinder, err)
				}
				klog.Infof("Found parent of supervised process: name - %s, pid - %d", cfg.PidFinder.parentName, parentPid)
			}

		}

		//Here we should have valid parent process if it specified
		cfg.PidFinder.svcProc, cfg.PidFinder.svcName, servicePid, err = findProcess(procs, cfg.PidFinder.svcPatternRegexp, cfg.PidFinder.parentProc)
		if err != nil {
			return cfg, fmt.Errorf("Can't find process %v: %v", cfg.PidFinder, err)
		}
		klog.Infof("Found supervised process: name - %s, pid - %d", cfg.PidFinder.svcName, servicePid)
	}

	return cfg, nil
}

func parseCommand(genericCfgParameter, genericCfgValue, commandType string, cfgParameters map[string]string, invertCommand bool) (string, error) {
	parameterKey := ""
	bValue := false

	switch commandType {
	case cmdAuto:
		//calculate bValue,
		switch genericCfgValue {
		case "true":
			bValue = true == !invertCommand
		case "false":
			bValue = false == !invertCommand
		default:
			return "", fmt.Errorf("key %q, unknown value: %q. Valid values: 'true', 'false'", genericCfgParameter, genericCfgValue)
		}

		switch bValue {
		case true:
			parameterKey = genericCfgParameter + "." + cmdEnable
		case false:
			parameterKey = genericCfgParameter + "." + cmdDisable
		}
	default:
		parameterKey = genericCfgParameter + "." + commandType
	}

	if command, ok := cfgParameters[parameterKey]; ok {
		return command, nil
	}
	return "", fmt.Errorf("paramter key %q not found in %s", parameterKey, cfgParameters)
}
