# nfs-provisioner
nfs-provisioner is an out-of-tree dynamic provisioner for Kubernetes. It automatically creates NFS `PersistentVolumes` for `PersistentVolumeClaims` that request a `StorageClass` configured to use some instance of nfs-provisioner as their provisioner. For more information see http://kubernetes.io/docs/user-guide/persistent-volumes/ and https://github.com/kubernetes/kubernetes/pull/30285.

## Deployment
You can run nfs-provisioner in Kubernetes as a pod or outside of Kubernetes as either a standalone binary or container.

Regardless of how it is run, you must decide on a unique name to give the provisioner that follows the naming scheme `<vendor name>/<provisioner name>` and pass it in with the `provisioner` argument. The provisioner will only provision volumes for claims that request a `StorageClass` with a provisioner field set equal to this name.

>Currently, by default the provisioner creates the NFS shares that back provisioned `PersistentVolumes` by making unique, deterministically named directories in `/exports` for each volume and exporting each made directory by doing `exportfs -o`. Unless `/exports` is persistent or some persistent storage is mounted there, the data will be gone when the provisioner stops. Even if the data is persistent, the provisioner won't attempt to recovery by searching for the directories and re-exporting them (yet?). How all this will work with persistent storage is still very much WIP :) So for now, PVs and PVCs may be left hanging.

### In Kubernetes

Build nfs-provisioner and a Docker image for it.

```
$ make container
```

Edit the `provisioner` argument in the `args` field in `deploy/kube-config/pod.yaml` to be the provisioner's name you decided on. 

```
$ kubectl create -f deploy/kube-config/pod.yaml
pod "nfs-provisioner" created
```

### Outside of Kubernetes - container

The container is going to need to run with `out-of-cluster` set true and one of `master` or `kubeconfig` set. For the `kubeconfig` argument to work, the config file needs to be inside the container somehow. This can be done by copying the kubeconfig file into the folder where the Dockerfile is and adding a line like `COPY config /config` to the Dockerfile.

Build nfs-provisioner and a Docker image for it.

```
$ make container
```

Run it with `provisioner` equal to the name you decided on, `out-of-cluster` set true and one of `master` or `kubeconfig` set. It needs to be run with Docker's `privileged` flag.

```
$ docker run --privileged wongma7/nfs-provisioner:latest -provisioner=matthew/nfs -out-of-cluster=true -kubeconfig=/config
```

### Outside of Kubernetes - binary TODO

Build nfs-provisioner.

```
$ make build
```

Run it with `provisioner` equal to the name you decided on, `out-of-cluster` set true, one of `master` or `kubeconfig` set, and `run-server` set to your preference. If you want the provisioner to be responsible for running the NFS server, leave `run-server` as true. Otherwise, it will assume the NFS server is running on the host somehow when it executes `exportfs -o` to export shares it creates. It probably needs to be run as root.

```
$ sudo ./nfs-provisioner -provisioner=matthew/nfs -out-of-cluster=true -kubeconfig=/home/matthew/.kube/config -run-server=false
```

### Arguments 
* `provisioner` - Name of the provisioner. The provisioner will only provision volumes for claims that request a StorageClass with a provisioner field set equal to this name.
* `out-of-cluster` - If the provisioner is being run out of cluster. Set the master or kubeconfig flag accordingly if true. Default false.
* `master` - Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.
* `kubeconfig` - Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.
* `run-server` - If the provisioner is responsible for running the NFS server, i.e. starting and stopping of the NFS server daemons. Default true.

## Usage

The nfs-provisioner has been deployed and is now watching for claims it should provision volumes for. No such claims can exist until a properly configured `StorageClass` for claims to request is created.

Edit the `provisioner` field in `deploy/kube-config/class.yaml` to be the provisioner's name. The nfs-provisioner as written doesn't take any `parameters` and will be unable to provision if any are specified, so don't specify any. Name the `StorageClass` however you like; the name is how claims will request this class. Create the class.
 
```
$ kubectl create -f deploy/kube-config/class.yaml
storageclass "matthew" created
```

Now if everything is working correctly, when you create a claim requesting the class you just created, the provisioner will automatically create a volume.

Edit the `volume.beta.kubernetes.io/storage-class` annotation in `deploy/kube-config/claim.yaml` to be the name of the class. Create the claim.

```
$ kubectl create -f deploy/kube-config/claim.yaml
persistentvolumeclaim "nfs" created
```

The nfs-provisioner provisions a PV for the PVC you just created.

```
$ kubectl get pv
NAME                                       CAPACITY   ACCESSMODES   RECLAIMPOLICY   STATUS      CLAIM         REASON    AGE
pvc-dce84888-7a9d-11e6-b1ee-5254001e0c1b   1Mi        RWX           Delete          Bound       default/nfs             23s
```

### Using as default

The provisioner can be used as the default storage provider, meaning claims that don't request a `StorageClass` get volumes provisioned for them by the provisioner by default. To set as the default a `StorageClass` that specifies the provisioner, turn on the `DefaultStorageClass` admission-plugin and add the `storageclass.beta.kubernetes.io/is-default-class` annotation to the class. See http://kubernetes.io/docs/user-guide/persistent-volumes/#class-1 for more information.

## Running Multiple Provisioners
TODO ... not much reason to talk about this until persistent stuff is figured out.

It's possible to run more than one instance of nfs-provisioner. There are a couple of things to note.
### Single StorageClass
(This is the same as having multiple `StorageClasses` all using the same nfs-provisioner as their provisioner, but there isn't much reason to do that since the provisioner doesn't take any parameters to differentiate between different `StorageClasses`.)

### Multiple StorageClasses
