package kvfs

import "container/list"

// cacheKey identifies one chunk across every file a VFS serves.
type cacheKey struct {
	name  string
	index uint32
}

// chunk is one cached piece of a file. Chunks live only in the cache: a file
// keeps the set of chunks it has written and asks the cache for them by key.
type chunk struct {
	key   cacheKey
	data  []byte
	file  *file
	dirty bool
	elem  *list.Element
}

// cache is an LRU of chunk buffers. Its budget is in bytes of file data, and
// evicting a dirty chunk writes it back to the store first: the alternative
// (dropping the write) would lose data SQLite believes it has handed over.
type cache struct {
	budget    int64
	used      int64
	chunkSize int64
	ll        *list.List
	items     map[cacheKey]*chunk
}

func newCache(budget int, chunkSize int64) *cache {
	if int64(budget) < chunkSize {
		budget = int(chunkSize)
	}
	return &cache{
		budget:    int64(budget),
		chunkSize: chunkSize,
		ll:        list.New(),
		items:     map[cacheKey]*chunk{},
	}
}

func (c *cache) get(key cacheKey) (*chunk, bool) {
	ch, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(ch.elem)
	return ch, true
}

// put inserts a chunk, evicting least-recently-used chunks until the budget
// fits. It returns an error if an eviction cannot be written back, leaving the
// cache unchanged: the caller still holds the chunk it just read or built.
func (c *cache) put(ch *chunk) error {
	if _, ok := c.items[ch.key]; ok {
		return nil
	}
	ch.elem = c.ll.PushFront(ch)
	c.items[ch.key] = ch
	c.used += c.chunkSize
	var evicted []*chunk
	defer func() {
		for _, e := range evicted {
			e.data = nil
			e.elem = nil
		}
	}()
	for c.used > c.budget {
		back := c.ll.Back()
		if back == nil || back == ch.elem {
			// Nothing but the chunk just inserted; a budget below one chunk
			// is nonsense but must not spin.
			break
		}
		victim := back.Value.(*chunk)
		if victim.dirty {
			if err := c.writeback(victim); err != nil {
				return err
			}
		}
		c.remove(victim)
		evicted = append(evicted, victim)
	}
	return nil
}

// writeback hands a dirty chunk to the store and clears its dirty flag.
func (c *cache) writeback(ch *chunk) error {
	if err := ch.file.write([]Mutation{{Key: chunkKey(ch.key.name, ch.key.index), Value: ch.data}}); err != nil {
		return err
	}
	ch.dirty = false
	delete(ch.file.dirty, ch.key.index)
	return nil
}

func (c *cache) remove(ch *chunk) {
	if _, ok := c.items[ch.key]; !ok {
		return
	}
	delete(c.items, ch.key)
	c.ll.Remove(ch.elem)
	c.used -= c.chunkSize
}

// forget drops every chunk of one file, for when the file is deleted.
func (c *cache) forget(name string) {
	for _, ch := range c.items {
		if ch.key.name == name {
			c.remove(ch)
			ch.data = nil
			ch.elem = nil
		}
	}
}

// dropFile drops the chunks that belong to one open file, for when that file
// is closed.
func (c *cache) dropFile(file *file) {
	for _, ch := range c.items {
		if ch.file == file {
			c.remove(ch)
			ch.data = nil
			ch.elem = nil
		}
	}
}
