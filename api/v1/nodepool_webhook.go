/*
Copyright 2022.

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

package v1

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	nodepoollog = logf.Log.WithName("nodepool-resource")
	keyReg      = regexp.MustCompile(`^node-pool.webapp.elasticweb-operator/*[a-zA-z0-9]*$`)
)

func (r *NodePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-webapp-elasticweb-operator-v1-nodepool,mutating=true,failurePolicy=fail,sideEffects=None,groups=webapp.elasticweb-operator,resources=nodepools,verbs=create;update,versions=v1,name=mnodepool.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &NodePool{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *NodePool) Default() {
	nodepoollog.Info("default", "name", r.Name)

	if len(r.Labels) == 0 {
		r.Labels["node-pool.webapp.elasticweb-operator"] = r.Name
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-webapp-elasticweb-operator-v1-nodepool,mutating=false,failurePolicy=fail,sideEffects=None,groups=webapp.elasticweb-operator,resources=nodepools,verbs=create;update,versions=v1,name=vnodepool.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &NodePool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *NodePool) ValidateCreate() error {
	nodepoollog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *NodePool) ValidateUpdate(old runtime.Object) error {
	nodepoollog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *NodePool) ValidateDelete() error {
	nodepoollog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *NodePool) validate() error {
	err := errors.Errorf("taint or label key must validatedy by %s", keyReg.String())

	for k := range r.Spec.Labels {
		if !keyReg.MatchString(k) {
			return errors.WithMessagef(err, "label key: %s", k)
		}
	}

	for _, taint := range r.Spec.Taints {
		if !keyReg.MatchString(taint.Key) {
			return errors.WithMessagef(err, "taint key: %s", taint.Key)
		}
	}
	return nil
}
