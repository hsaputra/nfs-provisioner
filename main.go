package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"

	"github.com/wongma7/nfs-provisioner/controller"
	"github.com/wongma7/nfs-provisioner/server"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/util/validation"
	"k8s.io/client-go/1.4/pkg/util/validation/field"
	"k8s.io/client-go/1.4/pkg/util/wait"
	"k8s.io/client-go/1.4/rest"
	"k8s.io/client-go/1.4/tools/clientcmd"
)

var (
	provisioner  = flag.String("provisioner", "matthew/nfs", "Name of the provisioner. The provisioner will only provision volumes for claims that request a StorageClass with a provisioner field set equal to this name.")
	outOfCluster = flag.Bool("out-of-cluster", false, "If the provisioner is being run out of cluster. Set the master or kubeconfig flag accordingly if true. Default false.")
	master       = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig   = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
	runServer    = flag.Bool("run-server", true, "If the provisioner is responsible for running the NFS server, i.e. starting and stopping of the NFS server daemons. Default true.")
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	if errs := validateProvisioner(*provisioner, field.NewPath("provisioner")); len(errs) != 0 {
		glog.Errorf("Invalid provisioner specified: %v", errs)
	}
	glog.Infof("Provisioner %s specified", *provisioner)

	if *runServer {
		// Start the NFS server
		err := server.Start()
		if err != nil {
			stopServerAndExit()
		}

		// On interrupt or SIGTERM, stop the NFS server
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			stopServerAndExit()
		}()
	}

	var config *rest.Config
	var err error
	if *outOfCluster {
		config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Errorf("Failed to create config: %v", err)
		stopServerAndExit()
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Errorf("Failed to create client: %v", err)
		stopServerAndExit()
	}

	// TODO is this useful?
	// Statically provision NFS PVs specified in exports.json, if exists
	// err = provisionStatic(clientset, "/etc/config/exports.json")
	// if err != nil {
	// 	glog.Errorf("Error while provisioning static exports: %v", err)
	// }

	// Start the NFS controller which will dynamically provision NFS PVs
	nc := controller.NewNfsController(clientset, 15*time.Second, *provisioner)
	nc.Run(wait.NeverStop)
}

// validateProvisioner is taken from https://github.com/kubernetes/kubernetes/blob/release-1.4/pkg/apis/storage/validation/validation.go
// validateProvisioner tests if provisioner is a valid qualified name.
func validateProvisioner(provisioner string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(provisioner) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, provisioner))
	}
	if len(provisioner) > 0 {
		for _, msg := range validation.IsQualifiedName(strings.ToLower(provisioner)) {
			allErrs = append(allErrs, field.Invalid(fldPath, provisioner, msg))
		}
	}
	return allErrs
}

func stopServerAndExit() {
	if *runServer {
		server.Stop()
	}

	os.Exit(1)
}
