# docker-valkey-cluster

Docker image for LOCAL development with a valkey cluster based on the published valkey docker image.

Forked from [pvogel1967/valkey-redis-cluster](https://github.com/pvogel1967/valkey-redis-cluster) and modified to add support for optimized image size, graceful shutdown, and more.

***Note:*** This is _not_ intended for production use; it runs 6 nodes of a valkey cluster in a single container intended as a convenience for local development and testing of code that uses a real valkey cluster (i.e. AWS Elasticache/Valkey, etc.).  

Why not just run a single node for local development?  Because you'll miss bugs in your code (i.e. sending a pipeline of commands that reference keys that reside in different nodes) that only show up when you have a cluster.  

## Discussions, help, guides

Github have recently released their `Discussions` feature into beta for more repositories across the github space. This feature is enabled on this repo since a while back.

Becuase we now have this feature, the issues feature will NOT be a place where you can now ask general questions or need simple help with this repo and what it provides.

What can you expect to find in there?

 - A place where you can freely ask any question regarding this repo.
 - Ask questions like `how do i do X?`
 - General help with problems with this repo
 - Guides written by me or any other contributer with useful examples and ansers to commonly asked questions and how to resolve thos problems.
 - Approved answers to questions marked and promoted by me if help is provided by the community regarding some questions


## What this repo and container IS

This repo exists as a resource to make it quick and simple to get a valkey cluster up and running with no fuss or issues with mininal effort. The primary use for this container is to get a cluster up and running in no time that you can use for demo/presentation/development. It is not intended or built for anything else.  I'll try to publish a new version of the container when new versions of valkey are released.  If you need a specific version, feel free to check out the repo and change the Dockerfile to start from the valkey image you need.


## What this repo and container IS NOT

This container that i have built is not supposed to be some kind of production container or one that is used within any environment other than running locally on your machine. It is not meant to be run on kubernetes or in any other prod/stage/test/dev environment as a fully working commponent in that environment. If that works for you and your use-case then awesome. But this container will not change to fit any other primary solution than to be used locally on your machine.

If you are looking for something else or some production quality or kubernetes compatible solution then you are looking in the wrong repo. There is other projects or forks of this repo that is compatible for that situation/solution.

For all other purposes other than what has been stated you are free to fork and/or rebuild this container using it as a template for what you need.



## Valkey instances inside the container

The cluster is 6 valkey instances running with 3 master & 3 slaves, one slave for each master. They run on ports 7000 to 7005 by default.  It's quite easy to change the ports and number of nodes by changing the environment variables in the docker-compose file or by passing them in as arguments to the `docker run` command.

If the flag `-e "SENTINEL=true"` is passed there are 3 Sentinel nodes running on ports 5000 to 5002 matching cluster's master instances.




# Important for Mac users

If you are using this container to run a valkey cluster on your mac computer, then you need to configure the container to use another IP address for cluster discovery as it can't use the default discovery IP that is hardcoded into the container.

If you are using the docker-compose file to build the container, then you must export a environment variable on your machine before building the container.

```
# This will make valkey do cluster discovery and bind all nodes to ip 127.0.0.1 internally

export VALKEY_CLUSTER_IP=0.0.0.0
```

If you are downloading the container from dockerhub, you must add the internal IP environment variable to your `docker run` command.

```
docker run -e "IP=0.0.0.0" -p 7000-7005:7000-7005 pvogel/valkey-cluster:latest
```



# Usage


## Include sentinel instances

Sentinel instances is not enabled by default.

If running with plain docker send in `-e SENTINEL=true`.

When running with docker-compose set the environment variable on your system `VALKEY_USE_SENTINEL=true` and start your container.

    services:
      valkey-cluster:
        ...
      environment:
        SENTINEL: 'true'


## Change number of nodes

Be default, it is going to launch 3 masters with 1 slave per master. This is configurable through a number of environment variables:

| Environment variable | Default |
| -------------------- |--------:|
| `INITIAL_PORT`       |    7000 |
| `MASTERS`            |       3 |
| `SLAVES_PER_MASTER`  |       1 | 

Therefore, the total number of nodes (`NODES`) is going to be `$MASTERS * ( $SLAVES_PER_MASTER  + 1 )` and ports are going to range from `$INITIAL_PORT` to `$INITIAL_PORT + NODES - 1`.

At the docker-compose provided by this repository, ports 7000-7050 are already mapped to the hosts'. Either if you need more than 50 nodes in total or if you need to change the initial port number, you should override those values.

Also note that the number of sentinels (if enabled) is the same as the number of masters. The docker-compose file already maps ports 5000-5010 by default. You should also override those values if you have more than 10 masters.

    services:
      valkey-cluster:
        ...
      environment:
        INITIAL_PORT: 9000,
        MASTERS: 2,
        SLAVES_PER_MASTER: 2


## IPv6 support

By default, valkey instances will bind and accept requests from any IPv4 network.
This is configurable by an environment variable that specifies which address a valkey instance will bind to.
By using the IPv6 variant `::` as counterpart to IPv4s `0.0.0.0` an IPv6 cluster can be created.

| Environment variable | Default |
| -------------------- | ------: |
| `BIND_ADDRESS`       | 0.0.0.0 |

Note that Docker also needs to be [configured](https://docs.docker.com/config/daemon/ipv6/) for IPv6 support.
Unfortunately Docker does not handle IPv6 NAT so, when acceptable, `--network host` can be used.

    # Example using plain docker
    docker run -e "IP=::1" -e "BIND_ADDRESS=::" --network host grokzen/valkey-cluster:latest


## Build alternative valkey versions


### docker build

My github actions use docker buildx to build a linux/arm64 and linux/amd64 image for the current valkey version. That image is pushed to `docker hub` as pvogel/valkey-cluster:<valkey-version>-<git tag> and pvogel/valkey-cluster:latest.


# License

This repo is using the MIT LICENSE.  And is based on the original work by Johan Andersson in grokzen/redis-cluster.  

You can find it in the file [LICENSE](LICENSE)
