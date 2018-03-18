package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"mitraft/labrpc"

	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

const (
	LEADER    = 0
	CANDIDATE = 1
	FOLLOWER  = 2
)

const HEARTBEAT_TIME = 50

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

type LogEntry struct {
	Command interface{}
	Term    int
	Index   int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int // index into peers[]
	state     int
	voteCount int

	// Your data here.
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Persistent state on all servers.
	currentTerm int
	votedFor    int
	logs        []LogEntry

	// Volatile state on all servers.
	commitIndex int
	lastApplied int

	// Volatile stsate on leaders.
	nextIndex  []int
	matchIndex []int

	grantVoteCh chan bool
	beLeaderCh  chan bool
	heartbeatCh chan bool
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here.
	rf.mu.Lock()
	defer rf.mu.Unlock()
	term = rf.currentTerm
	isleader = (rf.state == LEADER)
	return term, isleader
}

func (rf *Raft) getLastLogEntry() LogEntry {
	return rf.logs[len(rf.logs)-1]
}

func getRandomExpireTime() time.Duration {
	return time.Duration(rand.Int63n(300-150)+150) * time.Millisecond
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here.
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
}

//
// example RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

//
// example RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	if args.Term > rf.currentTerm {
		rf.state = FOLLOWER
		rf.currentTerm = args.Term
		rf.votedFor = -1
	}

	if rf.votedFor != -1 && rf.votedFor != args.CandidateId {
		reply.Term = args.Term
		reply.VoteGranted = false
		return
	}

	lastLogTerm := rf.getLastLogEntry().Term
	lastLogIndex := rf.getLastLogEntry().Index

	if (args.LastLogTerm > lastLogTerm) || (args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex) {
		reply.Term = args.Term
		reply.VoteGranted = true
		rf.grantVoteCh <- true
		rf.votedFor = args.CandidateId
	} else {
		reply.Term = args.Term
		reply.VoteGranted = false
	}

}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if !ok {
		return ok
	}

	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.votedFor = -1
		rf.state = FOLLOWER
		return ok
	}

	if reply.VoteGranted && rf.state == CANDIDATE {
		rf.voteCount++
		if rf.voteCount > len(rf.peers)/2 {
			rf.beLeaderCh <- true
		}
	}

	return ok
}

func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	rf.heartbeatCh <- true

	if args.Term > rf.currentTerm {
		rf.state = FOLLOWER
		rf.votedFor = -1
		rf.currentTerm = args.Term
	}

	reply.Success = true
	reply.Term = rf.currentTerm
}

func (rf *Raft) sendAppendEntries(server int, args AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if !ok {
		return ok
	}

	if reply.Term > rf.currentTerm {
		rf.state = FOLLOWER
		rf.votedFor = -1
		rf.voteCount = 0
		return ok
	}

	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.logs = make([]LogEntry, 1)

	rf.state = FOLLOWER
	rf.voteCount = 0

	rf.grantVoteCh = make(chan bool, 1)
	rf.beLeaderCh = make(chan bool, 1)
	rf.heartbeatCh = make(chan bool, 1)

	// Your initialization code here.
	go func(rf *Raft) {
		for {
			switch rf.state {

			case LEADER:
				broadcastAppendEntries(rf)
				time.Sleep(time.Duration(HEARTBEAT_TIME) * time.Millisecond)

			case CANDIDATE:
				rf.mu.Lock()
				rf.currentTerm++
				rf.voteCount = 1
				rf.votedFor = rf.me
				rf.mu.Unlock()

				go func() {
					broadcastRequestVote(rf)
				}()

				select {
				case <-time.After(getRandomExpireTime()):
				case <-rf.beLeaderCh:
					rf.mu.Lock()
					rf.state = LEADER
					rf.mu.Unlock()
				case <-rf.heartbeatCh:
					rf.mu.Lock()
					rf.state = FOLLOWER
					rf.votedFor = -1
					rf.voteCount = 0
					rf.mu.Unlock()
				}

			case FOLLOWER:
				select {
				case <-rf.heartbeatCh:
				case <-time.After(getRandomExpireTime()):
					rf.mu.Lock()
					rf.state = CANDIDATE
					rf.voteCount = 0
					rf.votedFor = -1
					rf.mu.Unlock()
				case <-rf.grantVoteCh:
				}
			}
		}
	}(rf)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}

func broadcastAppendEntries(rf *Raft) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	go func() {
		for i := range rf.peers {
			if rf.me != i && rf.state == LEADER {
				args := AppendEntriesArgs{}
				reply := &AppendEntriesReply{}

				args.Term = rf.currentTerm
				args.LeaderId = rf.me
				args.PrevLogTerm = rf.getLastLogEntry().Term
				args.PrevLogIndex = rf.getLastLogEntry().Index

				go func(server int) {
					rf.sendAppendEntries(server, args, reply)
				}(i)
			}

		}
	}()
}

func broadcastRequestVote(rf *Raft) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	for i := range rf.peers {
		if i != rf.me && rf.state == CANDIDATE {
			args := RequestVoteArgs{}
			reply := &RequestVoteReply{}

			args.Term = rf.currentTerm
			args.CandidateId = rf.me
			args.LastLogTerm = rf.getLastLogEntry().Term
			args.LastLogIndex = rf.getLastLogEntry().Index

			go func(server int) {
				rf.sendRequestVote(server, args, reply)
			}(i)
		}
	}

}
