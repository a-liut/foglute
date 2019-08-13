package edgeusher

import "math/rand"

const (
	uidLength = 8
	charset   = "abcdefghijklmnopqrstuvwxyz"
)

type SymbolTable struct {
	table        map[string]string
	reverseTable map[string]string
}

func (t *SymbolTable) Add(name string) string {
	if uid, exists := t.reverseTable[name]; exists {
		return uid
	}

	uid := newUID()
	t.table[uid] = name
	t.reverseTable[name] = uid
	return uid
}

func (t *SymbolTable) GetByName(name string) string {
	return t.reverseTable[name]
}

func (t *SymbolTable) GetByUID(uid string) string {
	return t.table[uid]
}

func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		table:        make(map[string]string),
		reverseTable: make(map[string]string),
	}
}

func newUID() string {
	b := make([]byte, uidLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
