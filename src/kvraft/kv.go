package kvraft

type KVStateMachine interface {
	Get(key string) (string, Err)
	Put(key, value string) Err
	Append(key, value string) Err
}

type MemoryKV struct {
	KV map[string]string
}

func newMemoryKV() *MemoryKV {
	return &MemoryKV{make(map[string]string)}
}

func (mkv *MemoryKV) Get(key string) (string, Err) {
	if value, ok := mkv.KV[key]; ok {
		return value, OK
	}
	return "", ErrNoKey
}

func (mkv *MemoryKV) Put(key, value string) Err {
	mkv.KV[key] = value
	return OK
}

func (mkv *MemoryKV) Append(key, value string) Err {
	mkv.KV[key] += value
	return OK
}
