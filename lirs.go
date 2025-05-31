package xcache

import (
	"container/list"
	"time"
)

// LIRS implements Low Inter-reference Recency Set cache replacement algorithm
type LIRSCache struct {
	baseCache
	stackS      *list.List                // LIRS stack for managing access history
	queueQ      *list.List                // Queue for resident HIR blocks
	items       map[interface{}]*lirsItem // Map of all cached items
	lirCount    int                       // Current count of LIR blocks
	maxLirCount int                       // Maximum allowed LIR blocks (typically 99% of cache size)
	maxHirCount int                       // Maximum allowed HIR blocks (typically 1% of cache size)
}

// lirsItem represents a cache item in LIRS
type lirsItem struct {
	clock      Clock
	key        interface{}
	value      interface{}
	expiration *time.Time
	isLIR      bool          // true if Low Inter-reference Recency, false if High
	isResident bool          // true if the block is in cache
	stackElem  *list.Element // Element in stack S
	queueElem  *list.Element // Element in queue Q (for HIR blocks only)
}

// newLIRSCache creates a new LIRS cache
func newLIRSCache(cb *CacheBuilder) *LIRSCache {
	c := &LIRSCache{}
	buildCache(&c.baseCache, cb)

	// Initialize data structures
	c.stackS = list.New()
	c.queueQ = list.New()
	c.items = make(map[interface{}]*lirsItem)

	// Set LIR and HIR block limits (99% LIR, 1% HIR)
	c.maxLirCount = int(float64(c.size) * 0.99)
	if c.maxLirCount < 1 {
		c.maxLirCount = c.size - 1
	}
	c.maxHirCount = c.size - c.maxLirCount
	if c.maxHirCount < 1 {
		c.maxHirCount = 1
	}

	c.lirCount = 0
	c.loadGroup.cache = c
	return c
}

// IsExpired checks if an item is expired
func (it *lirsItem) IsExpired(now *time.Time) bool {
	if it.expiration == nil {
		return false
	}
	if now == nil {
		t := it.clock.Now()
		now = &t
	}
	return it.expiration.Before(*now)
}

// Set a new key-value pair
func (c *LIRSCache) Set(key, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.set(key, value)
	return err
}

// SetWithExpire sets a key-value pair with expiration
func (c *LIRSCache) SetWithExpire(key, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, err := c.set(key, value)
	if err != nil {
		return err
	}

	t := c.clock.Now().Add(expiration)
	item.(*lirsItem).expiration = &t
	return nil
}

// set internal method for setting values
func (c *LIRSCache) set(key, value interface{}) (interface{}, error) {
	var err error
	if c.serializeFunc != nil {
		value, err = c.serializeFunc(key, value)
		if err != nil {
			return nil, err
		}
	}

	// Check if item already exists
	if item, exists := c.items[key]; exists {
		// Update existing item
		item.value = value
		if c.expiration != nil {
			t := c.clock.Now().Add(*c.expiration)
			item.expiration = &t
		}
		c.accessItem(item)
		if c.addedFunc != nil {
			c.addedFunc(key, value)
		}
		return item, nil
	}

	// Check if we need to evict before adding new item
	residentCount := c.getResidentCount()
	if residentCount >= c.size {
		// Need to evict before adding
		c.evictLeastRecentItem()
	}

	// Create new item
	item := &lirsItem{
		clock:      c.clock,
		key:        key,
		value:      value,
		isResident: true,
	}

	if c.expiration != nil {
		t := c.clock.Now().Add(*c.expiration)
		item.expiration = &t
	}

	// Determine if this should be LIR or HIR
	if c.lirCount < c.maxLirCount {
		// Space available for LIR block
		item.isLIR = true
		c.lirCount++
	} else {
		// Make it HIR block
		item.isLIR = false
	}

	c.items[key] = item
	c.insertIntoStack(item)

	if !item.isLIR {
		c.insertIntoQueue(item)
	}

	if c.addedFunc != nil {
		c.addedFunc(key, value)
	}

	return item, nil
}

// accessItem handles access to an existing item
func (c *LIRSCache) accessItem(item *lirsItem) {
	if item.isLIR {
		// LIR block access - move to top of stack
		c.moveToStackTop(item)
		if c.isStackBottom(item) {
			c.pruneStack()
		}
	} else {
		// HIR block access
		if item.isResident {
			if item.stackElem != nil {
				// HIR block in stack - convert to LIR
				c.convertToLIR(item)
			} else {
				// HIR block not in stack - move to end of queue
				c.moveToQueueEnd(item)
			}
		} else {
			// Non-resident HIR block - this is critical for cache size control
			if item.stackElem != nil {
				// In stack - convert to LIR
				// But first ensure we have space
				residentCount := c.getResidentCount()
				if residentCount >= c.size {
					c.evictLeastRecentItem()
				}
				item.isResident = true
				c.convertToLIR(item)
			} else {
				// Not in stack - make resident HIR
				// Ensure we have space before making it resident
				residentCount := c.getResidentCount()
				if residentCount >= c.size {
					c.evictLeastRecentItem()
				}
				item.isResident = true
				c.insertIntoQueue(item)
			}
			c.insertIntoStack(item)
		}
	}
}

// convertToLIR converts an HIR block to LIR
func (c *LIRSCache) convertToLIR(item *lirsItem) {
	// Before converting to LIR, ensure we don't exceed cache limit
	// If we're at capacity, we need to evict something first
	residentCount := c.getResidentCount()
	if residentCount >= c.size {
		// Remove LIR block at bottom of stack to make room
		if bottom := c.getStackBottom(); bottom != nil && bottom.isLIR && bottom != item {
			c.convertToHIR(bottom)
			// Actually evict the HIR block we just created
			c.evictFromQ()
		}
	}

	item.isLIR = true
	c.lirCount++

	// Remove from queue
	if item.queueElem != nil {
		c.queueQ.Remove(item.queueElem)
		item.queueElem = nil
	}

	// Move to top of stack
	c.moveToStackTop(item)

	// Convert LIR block at bottom of stack to HIR if it's not the same item
	if bottom := c.getStackBottom(); bottom != nil && bottom != item && bottom.isLIR {
		c.convertToHIR(bottom)
	}

	c.pruneStack()
}

// convertToHIR converts an LIR block to HIR
func (c *LIRSCache) convertToHIR(item *lirsItem) {
	item.isLIR = false
	c.lirCount--

	// Remove from stack if it's at bottom
	if c.isStackBottom(item) {
		c.stackS.Remove(item.stackElem)
		item.stackElem = nil
	}

	// Add to queue if resident
	if item.isResident {
		c.insertIntoQueue(item)
	}
}

// insertIntoStack inserts item at top of stack
func (c *LIRSCache) insertIntoStack(item *lirsItem) {
	if item.stackElem != nil {
		c.stackS.Remove(item.stackElem)
	}
	item.stackElem = c.stackS.PushFront(item)
}

// moveToStackTop moves item to top of stack
func (c *LIRSCache) moveToStackTop(item *lirsItem) {
	if item.stackElem != nil {
		c.stackS.MoveToFront(item.stackElem)
	} else {
		c.insertIntoStack(item)
	}
}

// insertIntoQueue inserts item at end of queue
func (c *LIRSCache) insertIntoQueue(item *lirsItem) {
	if item.queueElem != nil {
		c.queueQ.Remove(item.queueElem)
	}
	item.queueElem = c.queueQ.PushBack(item)
}

// moveToQueueEnd moves item to end of queue
func (c *LIRSCache) moveToQueueEnd(item *lirsItem) {
	if item.queueElem != nil {
		c.queueQ.MoveToBack(item.queueElem)
	} else {
		c.insertIntoQueue(item)
	}
}

// evictFromQ evicts the HIR block at front of queue
func (c *LIRSCache) evictFromQ() {
	if c.queueQ.Len() == 0 {
		return
	}

	front := c.queueQ.Front()
	item := front.Value.(*lirsItem)

	// Remove from queue
	c.queueQ.Remove(front)
	item.queueElem = nil
	item.isResident = false

	// Call evicted function if set
	if c.evictedFunc != nil {
		c.evictedFunc(item.key, item.value)
	}
}

// getStackBottom returns the bottom item of stack
func (c *LIRSCache) getStackBottom() *lirsItem {
	if c.stackS.Len() == 0 {
		return nil
	}
	return c.stackS.Back().Value.(*lirsItem)
}

// isStackBottom checks if item is at bottom of stack
func (c *LIRSCache) isStackBottom(item *lirsItem) bool {
	if c.stackS.Len() == 0 || item.stackElem == nil {
		return false
	}
	return c.stackS.Back() == item.stackElem
}

// pruneStack removes HIR blocks from bottom of stack
func (c *LIRSCache) pruneStack() {
	for c.stackS.Len() > 0 {
		bottom := c.stackS.Back()
		item := bottom.Value.(*lirsItem)

		if item.isLIR {
			break // Stop when we reach an LIR block
		}

		// Remove HIR block from stack
		c.stackS.Remove(bottom)
		item.stackElem = nil
	}
}

// Get retrieves a value from the cache
func (c *LIRSCache) Get(key interface{}) (interface{}, error) {
	v, err := c.get(key, false)
	if err == ErrKeyNotFoundError {
		return c.getWithLoader(key, true)
	}
	return v, err
}

// GetIFPresent gets a value if present
func (c *LIRSCache) GetIFPresent(key interface{}) (interface{}, error) {
	v, err := c.get(key, false)
	if err == ErrKeyNotFoundError {
		return c.getWithLoader(key, false)
	}
	return v, err
}

// Peek returns the value for the specified key if it is present in the cache
// without updating any eviction algorithm statistics or positions.
// This is a pure read operation that does not affect cache state.
func (c *LIRSCache) Peek(key interface{}) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, ErrKeyNotFoundError
	}

	if !item.IsExpired(nil) && item.isResident {
		value := item.value
		if c.deserializeFunc != nil {
			c.mu.RUnlock()
			defer c.mu.RLock()
			return c.deserializeFunc(key, value)
		}
		return value, nil
	}

	return nil, ErrKeyNotFoundError
}

// get internal method for getting values
func (c *LIRSCache) get(key interface{}, onLoad bool) (interface{}, error) {
	v, err := c.getValue(key, onLoad)
	if err != nil {
		return nil, err
	}
	if c.deserializeFunc != nil {
		return c.deserializeFunc(key, v)
	}
	return v, nil
}

// getValue internal method
func (c *LIRSCache) getValue(key interface{}, onLoad bool) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		if !onLoad {
			c.stats.IncrMissCount()
		}
		return nil, ErrKeyNotFoundError
	}

	if !item.IsExpired(nil) && item.isResident {
		c.accessItem(item)
		if !onLoad {
			c.stats.IncrHitCount()
		}
		return item.value, nil
	}

	// Item expired or not resident
	if item.IsExpired(nil) {
		c.removeItem(item)
	}

	if !onLoad {
		c.stats.IncrMissCount()
	}
	return nil, ErrKeyNotFoundError
}

// getWithLoader loads value using loader function
func (c *LIRSCache) getWithLoader(key interface{}, isWait bool) (interface{}, error) {
	if c.loaderExpireFunc == nil {
		return nil, ErrKeyNotFoundError
	}

	value, _, err := c.load(key, func(v interface{}, expiration *time.Duration, e error) (interface{}, error) {
		if e != nil {
			return nil, e
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		item, err := c.set(key, v)
		if err != nil {
			return nil, err
		}
		if expiration != nil {
			t := c.clock.Now().Add(*expiration)
			item.(*lirsItem).expiration = &t
		}
		return v, nil
	}, isWait)

	if err != nil {
		return nil, err
	}
	return value, nil
}

// removeItem removes an item from cache
func (c *LIRSCache) removeItem(item *lirsItem) {
	// Remove from stack
	if item.stackElem != nil {
		c.stackS.Remove(item.stackElem)
		item.stackElem = nil
	}

	// Remove from queue
	if item.queueElem != nil {
		c.queueQ.Remove(item.queueElem)
		item.queueElem = nil
	}

	// Update LIR count
	if item.isLIR {
		c.lirCount--
	}

	// Remove from items map
	delete(c.items, item.key)

	// Call evicted function
	if c.evictedFunc != nil && item.isResident {
		c.evictedFunc(item.key, item.value)
	}
}

// Has checks if key exists
func (c *LIRSCache) Has(key interface{}) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := time.Now()
	return c.has(key, &now)
}

// has internal method for checking existence
func (c *LIRSCache) has(key interface{}, now *time.Time) bool {
	item, exists := c.items[key]
	if !exists {
		return false
	}
	return !item.IsExpired(now) && item.isResident
}

// Remove removes a key from cache
func (c *LIRSCache) Remove(key interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	c.removeItem(item)
	return true
}

// GetALL returns all key-value pairs
func (c *LIRSCache) GetALL(checkExpired bool) map[interface{}]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make(map[interface{}]interface{})
	now := time.Now()

	for k, item := range c.items {
		if item.isResident && (!checkExpired || c.has(k, &now)) {
			items[k] = item.value
		}
	}

	return items
}

// Keys returns all keys
func (c *LIRSCache) Keys(checkExpired bool) []interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys []interface{}
	now := time.Now()

	for k := range c.items {
		if !checkExpired || c.has(k, &now) {
			keys = append(keys, k)
		}
	}

	return keys
}

// Len returns the number of items
func (c *LIRSCache) Len(checkExpired bool) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !checkExpired {
		count := 0
		for _, item := range c.items {
			if item.isResident {
				count++
			}
		}
		return count
	}

	var length int
	now := time.Now()
	for k := range c.items {
		if c.has(k, &now) {
			length++
		}
	}

	return length
}

// Purge removes all items
func (c *LIRSCache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.purgeVisitorFunc != nil {
		for _, item := range c.items {
			if item.isResident {
				c.purgeVisitorFunc(item.key, item.value)
			}
		}
	}

	// Clear all data structures
	c.stackS = list.New()
	c.queueQ = list.New()
	c.items = make(map[interface{}]*lirsItem)
	c.lirCount = 0
}

// getResidentCount returns the number of resident items
func (c *LIRSCache) getResidentCount() int {
	count := 0
	for _, item := range c.items {
		if item.isResident {
			count++
		}
	}
	return count
}

// evictLeastRecentItem evicts the least recent item
func (c *LIRSCache) evictLeastRecentItem() {
	// First try to evict from HIR queue
	if c.queueQ.Len() > 0 {
		c.evictFromQ()
		return
	}

	// If no HIR items, evict LIR item from bottom of stack
	if bottom := c.getStackBottom(); bottom != nil && bottom.isLIR {
		c.removeItem(bottom)
	}
}
