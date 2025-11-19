/*
Copyright 2020 The Crossplane Authors.

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

package controller

import (
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane-contrib/provider-http/internal/controller/cluster"
	"github.com/crossplane-contrib/provider-http/internal/controller/namespaced"
)

// Setup creates all http controllers (both cluster and namespaced) with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	for _, setup := range []func(ctrl.Manager, controller.Options, time.Duration) error{
		cluster.Setup,
		namespaced.Setup,
	} {
		if err := setup(mgr, o, timeout); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated creates all http controllers with SafeStart capability (controllers start as their CRDs appear)
func SetupGated(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	for _, setup := range []func(ctrl.Manager, controller.Options, time.Duration) error{
		cluster.SetupGated,
		namespaced.SetupGated,
	} {
		if err := setup(mgr, o, timeout); err != nil {
			return err
		}
	}
	return nil
}
