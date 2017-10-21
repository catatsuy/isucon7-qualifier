#!/bin/bash -x

echo "start deploy" | notify_slack
GOPATH=$PWD GOOS=linux go build -v isubata
for server in app01 app02 app03
do
    echo ssh -e sudo systemctl stop isubata.golang.service
    echo scp ./isubata $server:/home/isucon/isubata/webapp/go/isubata
    echo rsync -ave ./views/ $server:/home/isucon/isubata/webapp/go/views/
    echo ssh -e sudo systemctl start isubata.golang.service
done

echo "finish deploy" | notify_slack
