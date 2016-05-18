#!/bin/bash -e

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
docker build -t armadillica/pillar-notifserv .

echo
echo Docker image is build. Now you can run it with:
echo docker run --rm --name notifserv --net host  armadillica/pillar-notifserv
echo
echo Or create a new container using:
echo docker create --name notifserv --net host  armadillica/pillar-notifserv
