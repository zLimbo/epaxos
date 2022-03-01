go build -o bin/master master/master.go
sshpass -p z scp bin/master z@219.228.148.45:~/epaxos/master
