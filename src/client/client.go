package main

import (
	"bufio"
	"epaxos/dlog"
	"epaxos/genericsmrproto"
	"epaxos/masterproto"
	"epaxos/state"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"runtime"
	"time"
)

var masterAddr *string = flag.String("maddr", "", "Master address. Defaults to localhost")
var masterPort *int = flag.Int("mport", 7087, "Master port.  Defaults to 7077.")
var nReq *int = flag.Int("q", 5000, "Total number of requests. Defaults to 5000.")
var writes *int = flag.Int("w", 100, "Percentage of updates (writes). Defaults to 100%.")
var noLeader *bool = flag.Bool("e", false, "Egalitarian (no leader). Defaults to false.")
var fast *bool = flag.Bool("f", false, "Fast Paxos: send message directly to all replicas. Defaults to false.")
var nRound *int = flag.Int("r", 1, "Split the total number of requests into this many rounds, and do rounds sequentially. Defaults to 1.")
var procs *int = flag.Int("p", 2, "GOMAXPROCS. Defaults to 2")
var check = flag.Bool("check", false, "Check that every expected reply was received exactly once.")
var eps *int = flag.Int("eps", 0, "Send eps more messages per round than the client will wait for (to discount stragglers). Defaults to 0.")
var conflicts *int = flag.Int("c", -1, "Percentage of conflicts. Defaults to 0%")
var s = flag.Float64("s", 2, "Zipfian s parameter")
var v = flag.Float64("v", 1, "Zipfian v parameter")

var ReplicaNum int

var successful []int

var rarray []int
var rsp []bool

func init() {
	log.SetFlags(log.Lshortfile | log.Ltime)
}

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(*procs)

	randObj := rand.New(rand.NewSource(42))
	zipf := rand.NewZipf(randObj, *s, *v, uint64(*nReq / *nRound + *eps))

	if *conflicts > 100 {
		log.Fatalf("Conflicts percentage must be between 0 and 100.\n")
	}

	master, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", *masterAddr, *masterPort))
	if err != nil {
		log.Fatalf("Error connecting to master\n")
	}

	rlReply := new(masterproto.GetReplicaListReply)
	err = master.Call("Master.GetReplicaList", new(masterproto.GetReplicaListArgs), rlReply)
	if err != nil {
		log.Fatalf("Error making the GetReplicaList RPC")
	}

	ReplicaNum = len(rlReply.ReplicaList)
	servers := make([]net.Conn, ReplicaNum)
	readers := make([]*bufio.Reader, ReplicaNum)
	replicaWriters := make([]*bufio.Writer, ReplicaNum)

	rarray = make([]int, *nReq / *nRound + *eps)
	keyArray := make([]int64, *nReq / *nRound + *eps)
	put := make([]bool, *nReq / *nRound + *eps)
	perReplicaCount := make([]int, ReplicaNum)
	test := make([]int, *nReq / *nRound + *eps)
	for i := 0; i < len(rarray); i++ {
		r := rand.Intn(ReplicaNum)
		rarray[i] = r
		if i < *nReq / *nRound {
			perReplicaCount[r]++
		}

		if *conflicts >= 0 {
			r = rand.Intn(100)
			if r < *conflicts {
				keyArray[i] = 42
			} else {
				keyArray[i] = int64(43 + i)
			}
			r = rand.Intn(100)
			if r < *writes {
				put[i] = true
			} else {
				put[i] = false
			}
		} else {
			keyArray[i] = int64(zipf.Uint64())
			test[keyArray[i]]++
		}
	}
	if *conflicts >= 0 {
		log.Println("Uniform distribution")
	} else {
		log.Println("Zipfian distribution:")
		//log.Println(test[0:100])
	}

	for i := 0; i < ReplicaNum; i++ {
		var err error
		servers[i], err = net.Dial("tcp", rlReply.ReplicaList[i])
		if err != nil {
			log.Printf("Error connecting to replica %d\n", i)
		}
		readers[i] = bufio.NewReader(servers[i])
		// todo zd 写入远程服务器？
		replicaWriters[i] = bufio.NewWriter(servers[i])
	}

	successful = make([]int, ReplicaNum)
	leader := 0

	if *noLeader == false {
		reply := new(masterproto.GetLeaderReply)
		if err = master.Call("Master.GetLeader", new(masterproto.GetLeaderArgs), reply); err != nil {
			log.Fatalf("Error making the GetLeader RPC\n")
		}
		leader = reply.LeaderId
		log.Printf("The leader is replica %d\n", leader)
	}

	var cmdIdInc int32 = 0
	done := make(chan bool, ReplicaNum)
	args := genericsmrproto.Propose{cmdIdInc, state.Command{Op: state.PUT, K: 0, V: 0}, 0}

	before_total := time.Now()

	nReqPerRound := *nReq / *nRound
	dlog.Printf("zd | reqsNb: %d, rounds: %d, nReqPerRound: %d", *nReq, *nRound, nReqPerRound)
	rsp = make([]bool, *nReq)
	delays := make([]time.Duration, *nRound)
	for round := 0; round < *nRound; round++ {
		if *check {
			for k := 0; k < nReqPerRound; k++ {
				rsp[round*nReqPerRound+k] = false
			}
		}

		// todo zd 无主？
		dlog.Printf("zd | isNoLeader: %v", *noLeader)
		if *noLeader {
			for i := 0; i < ReplicaNum; i++ {
				go waitReplies(readers, i, perReplicaCount[i], done)
			}
		} else {
			go waitReplies(readers, leader, nReqPerRound, done)
		}

		before := time.Now()

		for reqIdxPerRound := 0; reqIdxPerRound < nReqPerRound+*eps; reqIdxPerRound++ {
			dlog.Printf("Sending proposal %d\n", cmdIdInc)
			args.CommandId = cmdIdInc
			if put[reqIdxPerRound] {
				args.Command.Op = state.PUT
			} else {
				args.Command.Op = state.GET
			}
			args.Command.K = state.Key(keyArray[reqIdxPerRound])
			args.Command.V = state.Value(reqIdxPerRound)
			//args.Timestamp = time.Now().UnixNano()
			if !*fast {
				if *noLeader {
					leader = rarray[reqIdxPerRound]
				}
				replicaWriters[leader].WriteByte(genericsmrproto.PROPOSE)
				args.Marshal(replicaWriters[leader])
			} else {
				//send to everyone
				for replicaIndex := 0; replicaIndex < ReplicaNum; replicaIndex++ {
					// todo zd 先写消息类型，后写数据
					replicaWriters[replicaIndex].WriteByte(genericsmrproto.PROPOSE)
					args.Marshal(replicaWriters[replicaIndex])
					replicaWriters[replicaIndex].Flush()
				}
			}
			//log.Println("Sent", id)
			cmdIdInc++
			if reqIdxPerRound%100 == 0 {
				for i := 0; i < ReplicaNum; i++ {
					replicaWriters[i].Flush()
				}
			}
		}
		for i := 0; i < ReplicaNum; i++ {
			replicaWriters[i].Flush()
		}

		err := false
		if *noLeader {
			for i := 0; i < ReplicaNum; i++ {
				e := <-done
				err = e || err
			}
		} else {
			err = <-done
		}

		after := time.Now()

		// 计算延时
		delays[round] = time.Since(before)

		log.Printf("Round %d, took %v, tx.num = %d", round, after.Sub(before), nReqPerRound)

		// todo 检查
		if *check {
			for k := 0; k < nReqPerRound; k++ {
				if !rsp[round*nReqPerRound+k] {
					log.Println("Didn't receive", k)
				}
			}
		}

		if err {
			if *noLeader {
				ReplicaNum = ReplicaNum - 1
			} else {
				reply := new(masterproto.GetLeaderReply)
				master.Call("Master.GetLeader", new(masterproto.GetLeaderArgs), reply)
				leader = reply.LeaderId
				log.Printf("New leader is replica %d\n", leader)
			}
		}
	}

	after_total := time.Now()
	log.Printf("Test took %v\n", after_total.Sub(before_total))

	s := 0
	for _, succ := range successful {
		s += succ
	}

	log.Printf("Successful: %d\n", s)

	spend := ToSecond(time.Since(before_total))
	tps := float64(*nReq) / spend

	tmpTotal := time.Duration(0)
	for _, delay := range delays {
		tmpTotal += delay
	}
	avgDelay := ToSecond(tmpTotal) / float64(len(delays)) * 1000

	log.Printf("tx.num: %d, batch: %d, spend: %.2f(s), tps: %.2f, avgDelay: %.2f(ms)", *nReq, *nRound, spend, tps, avgDelay)

	fmt.Println("tx.size\ttx.num\tbatch\tbatch.tx.num\tbatch.size\tspend\ttps\tavgDelay")
	fmt.Printf("%d\t%d\t%d\t%d\t%.2f\t%.2f\t%.2f\t%.2f\t\n",
		state.TxSize,
		*nReq,
		*nRound,
		nReqPerRound,
		float64(state.TxSize*nReqPerRound)/1024/1024,
		spend,
		tps,
		avgDelay)

	for _, client := range servers {
		if client != nil {
			client.Close()
		}
	}
	master.Close()
}

func ToSecond(td time.Duration) float64 {
	return float64(td.Nanoseconds()) / math.Pow10(9)
}

func waitReplies(readers []*bufio.Reader, leader int, n int, done chan bool) {
	e := false

	reply := new(genericsmrproto.ProposeReplyTS)
	for i := 0; i < n; i++ {
		if err := reply.Unmarshal(readers[leader]); err != nil {
			log.Println("Error when reading:", err)
			e = true
			continue
		}
		//log.Println(reply.Value)
		if *check {
			if reply.CommandId >= int32(len(rsp)) {
				log.Printf("zd | index out: reply.CommandId = %d, rsp.len = %d", reply.CommandId, len(rsp))
			} else {
				if rsp[reply.CommandId] {
					log.Println("Duplicate reply", reply.CommandId)
				}
				rsp[reply.CommandId] = true
			}
		}
		if reply.OK != 0 {
			successful[leader]++
		}
	}
	done <- e
}
