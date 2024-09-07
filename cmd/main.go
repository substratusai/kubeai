/*
Copyright 2024.

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
	"os"

	"github.com/substratusai/kubeai/internal/command"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	sysCfg, err := command.LoadConfigFile(configPath)
	if err != nil {
		command.Log.Error(err, "failed to load config file", "path", configPath)
		os.Exit(1)
	}

	if err := command.Run(ctrl.SetupSignalHandler(), ctrl.GetConfigOrDie(), sysCfg); err != nil {
		command.Log.Error(err, "failed to run command")
		os.Exit(1)
	}
}
