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

package cluster

import (
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	clusterdisposablerequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/disposablerequest/v1alpha2"
	clusterrequestv1alpha2 "github.com/crossplane-contrib/provider-http/apis/cluster/request/v1alpha2"
	httpv1alpha1 "github.com/crossplane-contrib/provider-http/apis/cluster/v1alpha1"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/config"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/disposablerequest"
	"github.com/crossplane-contrib/provider-http/internal/controller/cluster/request"
)

// Setup creates all cluster-scoped http controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	for _, setup := range []func(ctrl.Manager, controller.Options, time.Duration) error{
		config.Setup,
		disposablerequest.Setup,
		request.Setup,
	} {
		if err := setup(mgr, o, timeout); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated creates all cluster-scoped http controllers with SafeStart capability (controllers start as their CRDs appear)
func SetupGated(mgr ctrl.Manager, o controller.Options, timeout time.Duration) error {
	o.Gate.Register(func() {
		if err := config.Setup(mgr, o, timeout); err != nil {
			panic(err)
		}
	}, httpv1alpha1.ProviderConfigGroupVersionKind)

	o.Gate.Register(func() {
		if err := disposablerequest.Setup(mgr, o, timeout); err != nil {
			panic(err)
		}
	}, clusterdisposablerequestv1alpha2.DisposableRequestGroupVersionKind)

	o.Gate.Register(func() {
		if err := request.Setup(mgr, o, timeout); err != nil {
			panic(err)
		}
	}, clusterrequestv1alpha2.RequestGroupVersionKind)

	return nil
}
