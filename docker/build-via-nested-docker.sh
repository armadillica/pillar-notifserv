#!/bin/bash -e

HASH=$(git show-ref --head --hash HEAD)
EXPORT_TO=pillar-notifserv-${HASH}.docker.tgz

# Use Docker to get Go in a way that allows overwriting the
# standard library with statically linked versions.
docker run -i --rm \
    -v $(pwd):/docker \
    -v "${GOPATH}:/go-local" \
    --env GOPATH=/go-local \
     google/golang /bin/bash -ex << EOT
CGO_ENABLED=0 go get -a -ldflags '-s' github.com/armadillica/pillar-notifserv
cp \${GOPATH}/bin/pillar-notifserv /docker
EOT

# Use the statically linked executable to build our final Docker image.
docker build -t armadillica/pillar-notifserv:${HASH} .

if docker ps -a --no-trunc | grep -q notifserv; then
    echo
    echo '==> Docker container "notifserv" already exists, press ENTER to remove and recreate.'
    read dummy
    docker stop notifserv
    docker rm notifserv
fi

docker save armadillica/pillar-notifserv:${HASH} | gzip > ${EXPORT_TO}
echo Docker container created and exported to ${EXPORT_TO}

cat > pillar-notifserv-install-${HASH}.sh << EOT
#!/bin/sh
gunzip < pillar-notifserv-${HASH}.docker.tgz | docker load
echo
echo Image installed, create container with:
echo docker create --name notifserv --net host  armadillica/pillar-notifserv:${HASH}
EOT
chmod +x pillar-notifserv-install-${HASH}.sh
