go build -o bin/client client/client.go
sshpass -p z scp bin/client z@219.228.148.172:~/epaxos/client
