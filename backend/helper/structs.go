package helper 

type Node struct {
    Id int
    ChordSize int
    AmountOfNodes int
    Address string
    FingerTable []FingerTableEntry //need this for effective joining of new nodes 
    Neighbours Neighbours
    SeedToReceiveFrom RTTEntry
    Seeds []string //keep a list of the ip addresses of the seeds :) 
    Seed bool
    NodesToSendTo []RTTEntry
}


type RTTEntry struct {
    Address string
    RTTTime uint64
}

type FingerTableEntry struct {
    Key int
    NodeId int
    NodeAddress string
}


type Neighbours struct {
    Predecessor *Node
    Successor *Node
}
