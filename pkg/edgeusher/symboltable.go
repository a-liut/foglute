/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
package edgeusher

import "math/rand"

const (
	uidLength = 8
	charset   = "abcdefghijklmnopqrstuvwxyz" // Charset used for generating UIDs
)

// The SymbolTable is used to store mappings between names and UIDs when converting EdgeUsher objects
type SymbolTable struct {
	table        map[string]string
	reverseTable map[string]string
}

// Add a new name into the table.
// It returns the UID associated to the name.
// If the names is already in the table, the old UID is returned, otherwise a new UID is generated.
func (t *SymbolTable) Add(name string) string {
	if uid, exists := t.reverseTable[name]; exists {
		return uid
	}

	uid := newUID()
	t.table[uid] = name
	t.reverseTable[name] = uid
	return uid
}

// Returns the UID of a name
func (t *SymbolTable) GetByName(name string) string {
	return t.reverseTable[name]
}

// Returns the name associated to an UID
func (t *SymbolTable) GetByUID(uid string) string {
	return t.table[uid]
}

// Returns a new SymbolTable instance
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		table:        make(map[string]string),
		reverseTable: make(map[string]string),
	}
}

// Generates a new random UID
func newUID() string {
	b := make([]byte, uidLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
