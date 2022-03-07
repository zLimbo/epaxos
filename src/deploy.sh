#!/bin/bash

master="219.228.148.45"
client="219.228.148.231"
servers=(
    "219.228.148.80"
    "219.228.148.89"
    "219.228.148.129"
    "219.228.148.169"
    "219.228.148.172"
    "219.228.148.181"
    "219.228.148.222"
)

function deployMaster() {
    printf "\n[deployMaster]\n"
    go build -o bin/master master/master.go

    printf "deploy master in %-16s ..." ${master}
    start=$(date +%s)
    sshpass -p z scp bin/master z@${master}:~/epaxos/master
    end=$(date +%s)
    take=$((end - start))
    printf "\rdeploy master in %-16s ok, take %ds\n" ${master} ${take}
}

function deployClient() {
    printf "\n[deployClient]\n"
    go build -o bin/client client/client.go
    printf "deploy client in %-16s ..." ${client}

    start=$(date +%s)
    sshpass -p z scp bin/client z@${client}:~/epaxos/client
    end=$(date +%s)
    take=$((end - start))
    printf "\rdeploy client in %-16s ok, take %ds\n" ${client} ${take}
}

function deployServer() {
    printf "\n[deployServer]\n"
    go build -o bin/server server/server.go

    for srv in ${servers[@]}; do
        printf "deploy server in %-16s ..." ${srv}
        start=$(date +%s)
        sshpass -p z scp bin/server z@${srv}:~/epaxos/server
        end=$(date +%s)
        take=$((end - start))
        printf "\rdeploy server in %-16s ok, take %ds\n" ${srv} ${take}
    done
}

if (($# == 0)); then
    echo
    echo "please input 'master', 'client', 'server' or 'all' !"
    exit
fi

if [ $1 == "a" ]; then
    deployMaster
    deployClient
    deployServer
elif [ $1 == "m" ]; then
    deployMaster
elif [ $1 == "c" ]; then
    deployClient
elif [ $1 == "s" ]; then
    deployServer
else
    echo "please input 'm', 'c', 's' or 'a' !"
fi
