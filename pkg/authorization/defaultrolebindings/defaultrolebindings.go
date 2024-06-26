package defaultrolebindings

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// RoleBindingController is a controller to combine cluster roles
type RoleBindingController struct {
	name              string
	roleBindingClient rbacclient.RoleBindingsGetter

	roleBindingLister rbaclisters.RoleBindingLister
	roleBindingSynced cache.InformerSynced
	namespaceLister   corelisters.NamespaceLister
	namespaceSynced   cache.InformerSynced

	syncHandler      func(namespace string) error
	queue            workqueue.RateLimitingInterface
	roleBindingsFunc projectRoleBindings
}

// NewRoleBinding creates a new controller
func NewRoleBindingsController(roleBindingInformer rbacinformers.RoleBindingInformer, namespaceInformer coreinformers.NamespaceInformer, roleBindingClient rbacclient.RoleBindingsGetter, controllerName string) *RoleBindingController {
	c := &RoleBindingController{
		name:              controllerName,
		roleBindingClient: roleBindingClient,

		roleBindingLister: roleBindingInformer.Lister(),
		roleBindingSynced: roleBindingInformer.Informer().HasSynced,
		namespaceLister:   namespaceInformer.Lister(),
		namespaceSynced:   namespaceInformer.Informer().HasSynced,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	c.syncHandler = c.syncNamespace
	c.roleBindingsFunc = GetRoleBindingsForController(controllerName)

	roleBindingNames := GetBootstrapServiceAccountProjectRoleBindingNames(c.roleBindingsFunc)

	roleBindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			metadata, err := meta.Accessor(obj)
			if err != nil {
				return false
			}
			return roleBindingNames.Has(metadata.GetName())
		},
		Handler: cache.ResourceEventHandlerFuncs{
			DeleteFunc: func(uncast interface{}) {
				metadata, err := meta.Accessor(uncast)
				if err == nil {
					c.queue.Add(metadata.GetNamespace())
					return
				}

				tombstone, ok := uncast.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", uncast))
					return
				}
				metadata, err = meta.Accessor(tombstone.Obj)
				if err != nil {
					utilruntime.HandleError(err)
					return
				}
				c.queue.Add(metadata.GetNamespace())
			},
		},
	})
	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			metadata, err := meta.Accessor(obj)
			if err != nil {
				utilruntime.HandleError(err)
				return
			}
			c.queue.Add(metadata.GetName())
		},
	})
	return c
}

func (c *RoleBindingController) syncNamespace(namespaceName string) error {
	namespace, err := c.namespaceLister.Get(namespaceName)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if namespace.DeletionTimestamp != nil {
		return nil
	}

	roleBindings, err := c.roleBindingLister.RoleBindings(namespaceName).List(labels.Everything())
	if err != nil {
		return err
	}

	errs := []error{}
	desiredRoleBindings := GetRoleBindingsForController(c.name)(namespaceName)

	for i := range desiredRoleBindings {
		desiredRoleBinding := desiredRoleBindings[i]
		found := false
		for _, existingRoleBinding := range roleBindings {
			if existingRoleBinding.Name == desiredRoleBinding.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}

		_, err := c.roleBindingClient.RoleBindings(namespaceName).Create(context.TODO(), &desiredRoleBinding, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return utilerrors.NewAggregate(errs)
}

// Run starts the controller and blocks until stopCh is closed.
func (c *RoleBindingController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v", c.name)
	defer klog.Infof("Shutting down %v", c.name)

	if !cache.WaitForNamedCacheSync(c.name, stopCh, c.roleBindingSynced, c.namespaceSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *RoleBindingController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *RoleBindingController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.syncHandler(dsKey.(string))
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}
