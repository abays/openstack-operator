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

package core

import (
	"context"
	"fmt"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	ovsv1 "github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// GetClient -
func (r *OpenStackControlPlaneReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackControlPlaneReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackControlPlaneReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackControlPlaneReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// OpenStackControlPlaneReconciler reconciles a OpenStackControlPlane object
type OpenStackControlPlaneReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
	Log     logr.Logger
}

//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=placement.openstack.org,resources=placementapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cinder.openstack.org,resources=cinders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nova.openstack.org,resources=nova,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=neutron.openstack.org,resources=neutronapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ovn.openstack.org,resources=ovndbclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ovn.openstack.org,resources=ovnnorthds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ovs.openstack.org,resources=ovs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the OpenStackControlPlane instance
	instance := &corev1beta1.OpenStackControlPlane{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		// Register the finalizer immediately to avoid orphaning resources on delete
		err := r.Update(ctx, instance)

		return ctrl.Result{}, err
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneRabbitMQReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneRabbitMQReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneOVNReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneOVNReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneOVSReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneOVSReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneNeutronReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneNeutronReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneMariaDBReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneMariaDBReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneKeystoneAPIReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlanePlacementAPIReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneGlanceReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneGlanceReadyInitMessage),
			// TODO add once rabbitmq transportURL is integrated with Cinder: condition.UnknownCondition(corev1beta1.OpenStackControlPlaneCinderReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneCinderReadyInitMessage),
			condition.UnknownCondition(corev1beta1.OpenStackControlPlaneNovaReadyCondition, condition.InitReason, corev1beta1.OpenStackControlPlaneNovaReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, r.Status().Update(ctx, instance)
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the overall status condition if service is ready
		if instance.IsReady() {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		if err := helper.SetAfter(instance); err != nil {
			util.LogErrorForObject(helper, err, "Set after and calc patch/diff", instance)
		}

		if changed := helper.GetChanges()["status"]; changed {
			patch := client.MergeFrom(helper.GetBeforeObject())

			if err := r.Status().Patch(ctx, instance, patch); err != nil && !k8s_errors.IsNotFound(err) {
				util.LogErrorForObject(helper, err, "Update status", instance)
			}
		}
	}()

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	return r.reconcileNormal(ctx, instance, helper)
}

func (r *OpenStackControlPlaneReconciler) reconcileNormal(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

	ctrlResult, err := openstack.ReconcileRabbitMQ(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileMariaDB(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileKeystoneAPI(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcilePlacementAPI(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileGlance(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileCinder(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOVN(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOVS(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileNeutron(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileNova(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackControlPlaneReconciler) reconcileDelete(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	var err error
	overallRes := ctrl.Result{}

	// Delete non-Keystone, non-Rabbit services first
	res, err := openstack.DeleteCinder(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeleteGlance(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeleteNeutron(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeleteNova(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeleteOVN(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeleteOVS(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeletePlacement(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	// If we're still waiting for the resources above to be deleted, stop here
	if (overallRes != ctrl.Result{}) {
		return overallRes, nil
	}

	// Delete Rabbit and Keystone
	res, err = openstack.DeleteRabbitMq(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	res, err = openstack.DeleteKeystone(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	// If we're still waiting for the resources above to be deleted, stop here
	if (overallRes != ctrl.Result{}) {
		r.Log.Info(fmt.Sprintf("OpenStackControlPlane %s deletion is waiting on sub-resource deletion, requeuing...", instance.Name))
		return overallRes, nil
	}

	// Finally, delete MariaDB
	res, err = openstack.DeleteMariaDB(ctx, instance, helper)
	if err != nil {
		return res, err
	} else if (res != ctrl.Result{}) {
		overallRes = res
	}

	if (overallRes == ctrl.Result{}) {
		// Everything is cleared, so remove the finalizer so this OpenStackControlPlane can be fully removed
		controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
		if err := r.Update(ctx, instance); err != nil && !k8s_errors.IsNotFound(err) {
			return overallRes, err
		}

		r.Log.Info("Reconciled Service delete successfully")
	}

	return overallRes, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.OpenStackControlPlane{}).
		Owns(&mariadbv1.MariaDB{}).
		Owns(&keystonev1.KeystoneAPI{}).
		Owns(&placementv1.PlacementAPI{}).
		Owns(&glancev1.Glance{}).
		Owns(&cinderv1.Cinder{}).
		Owns(&rabbitmqv1.RabbitmqCluster{}).
		Owns(&ovnv1.OVNDBCluster{}).
		Owns(&ovnv1.OVNNorthd{}).
		Owns(&ovsv1.OVS{}).
		Owns(&neutronv1.NeutronAPI{}).
		Owns(&novav1.Nova{}).
		Complete(r)
}
