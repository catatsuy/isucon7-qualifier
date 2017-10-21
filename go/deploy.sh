#!/bin/bash -x

echo "start deploy" | notify_slack
GOPATH=$PWD GOOS=linux go build -v isubata
for server in app01 app02 app03
do
    ssh -t $server "sudo systemctl stop isubata.golang.service"
    scp ./isubata $server:/home/isucon/isubata/webapp/go/isubata
    rsync -av ./src/isubata/views/ $server:/home/isucon/isubata/webapp/go/src/isubata/views/
    ssh -t $server "sudo systemctl start isubata.golang.service"
done

echo "finish deploy" | notify_slack
