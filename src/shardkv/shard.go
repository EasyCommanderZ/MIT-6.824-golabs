package shardkv

type Shard struct {
	KV     map[string]string
	Status string
}

const (
	STATUS_SERVING   = "SERVING"
	STATUS_PULLING   = "PULLING"
	STATUS_BEPULLING = "BEPULLING"
	STATUS_GC        = "GC"
)

func NewShard() *Shard {
	return &Shard{
		KV:     make(map[string]string),
		Status: STATUS_SERVING,
	}
}

func (sh *Shard) Get(key string) (string, Err) {
	if value, ok := sh.KV[key]; ok {
		return value, OK
	}
	return "", ErrNoKey
}

func (sh *Shard) Put(key string, value string) Err {
	sh.KV[key] = value
	return OK
}

func (sh *Shard) Append(key string, value string) Err {
	sh.KV[key] += value
	return OK
}

func (sh *Shard) dcopy() map[string]string {
	newShardKV := make(map[string]string)
	for k, v := range sh.KV {
		newShardKV[k] = v
	}
	return newShardKV
}
