go build -o bin/server server/server.go
sshpass -p z scp bin/server z@219.228.148.80:~/epaxos/server
sshpass -p z scp bin/server z@219.228.148.89:~/epaxos/server
sshpass -p z scp bin/server z@219.228.148.172:~/epaxos/server
sshpass -p z scp bin/server z@219.228.148.231:~/epaxos/server
