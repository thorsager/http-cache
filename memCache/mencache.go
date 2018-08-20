package memCache

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

const sigInitialize int = 0
const sigPut int = 1
const sigGet int = 3
const sigDelete int = 4

type msg struct {
	sig int
	itemKey string
	time time.Time
}

type MemCache struct {
	MaxItemSize int
	MaxIdleTime int
	initialized bool
	mutex sync.RWMutex
	metaMutex sync.RWMutex
	items map[string][]byte
	meta map[string]itemInfo
	syncWG  sync.WaitGroup

	sideKick struct {
		msgChan chan msg
	}
	scheduler struct {
		quitChan chan int
	}
}

type itemInfo struct {
	accessCount int
	version int
	lastAccess time.Time
	lastUpdate time.Time
	created time.Time
}

func (c *MemCache)sidekick() {
	skLog := log.WithField("task","sideKick")
	skLog.Debug("Starting..")
	defer c.syncWG.Done()
	for {
		msg,more := <-c.sideKick.msgChan
		if !more {
			skLog.Debug("Terminating..")
			break
		}
		switch msg.sig {
		case sigInitialize:
			if c.meta == nil {
				c.meta = make(map[string]itemInfo)
				skLog.Debug("Initialized..")
			}
			break
		case sigDelete:
			c.metaMutex.Lock()
			delete(c.meta,msg.itemKey)
			c.metaMutex.Unlock()
			skLog.Debugf("Deleted %s",msg.itemKey)
			break
		case sigGet:
			c.metaMutex.Lock()
			itemMeta,ok :=c.meta[msg.itemKey]
			if ok {
				itemMeta.accessCount += 1
			}
			c.metaMutex.Unlock()
			if ok {
				skLog.Debugf("Updated accessCount on %s",msg.itemKey)
			}
			break
		case sigPut:
			c.metaMutex.Lock()
			if itemMeta,ok :=c.meta[msg.itemKey]; ok {
				itemMeta.version += 1
				itemMeta.lastUpdate = msg.time
			} else {
				c.meta[msg.itemKey] = itemInfo{version:0, lastUpdate:msg.time, accessCount:0}
			}
			c.metaMutex.Unlock()
			skLog.Debugf("Updated/Created %s @%d",msg.itemKey,msg.time)
			break
		default:
			skLog.Debugf("Got un-handled %d msg on %s",msg.sig,msg.itemKey)
		}
	}
}

func (c *MemCache)scheduleRunner() {
	schLog := log.WithField("task","scheduler")
	schLog.Debug("Starting..")
	defer c.syncWG.Done()

	run := true
	for {
		select {
		case _, more := <-c.scheduler.quitChan:
			if !more {
				schLog.Debug("Terminating..")
				run = false
				break
			}
		case <- time.After(500 * time.Millisecond):
			schLog.Debug("!!Tick!!")
		}

		if !run {
			break
			schLog.Debug("End-loop!!")
		}
	}
}


func (c *MemCache) Size() int {
	return len(c.items)
}

func (c *MemCache) Close() {
	if c.initialized {
		close(c.sideKick.msgChan)
		close(c.scheduler.quitChan)
		c.syncWG.Wait()

		c.mutex.Lock()
		for item := range c.items {
			delete(c.items, item)
		}
		c.mutex.Unlock()
		log.Info("Close() called")
	}
}

func (c *MemCache) Init() {
	if !c.initialized {
		c.initialized = true
		c.items = make(map[string][]byte)

		c.sideKick.msgChan = make(chan msg, 5)
		c.syncWG.Add(1)
		go c.sidekick()

		c.scheduler.quitChan = make(chan int)
		c.syncWG.Add(1)
		go c.scheduleRunner()

		log.Info("Init() called")
		c.sideKick.msgChan <- msg{sig:sigInitialize}

		if c.MaxItemSize == 0 {
			c.MaxItemSize = 50*1024  // Defaul to 50K
		}
	}
}

func (c *MemCache) Get(key string) (data []byte, found bool) {
	if !c.initialized {
		return nil,false
	}
	c.mutex.RLock()
	if item,ok := c.items[key]; ok {
		data = item
		found = true
	}
	c.mutex.RUnlock()
	c.sideKick.msgChan <- msg{sig:sigGet,itemKey:key}
	return data,found
}

func (c *MemCache) Put(key string, data []byte) (ok bool) {
	if c.initialized && c.WillAccept(&data) {
		c.mutex.Lock()
		c.items[key] = data
		c.sideKick.msgChan <- msg{sig: sigPut, itemKey: key, time:time.Now()}
		c.mutex.Unlock()
		return true
	}
	return false
}

func (c *MemCache) Delete(key string) {
	if c.initialized {
		c.mutex.Lock()
		delete(c.items, key)
		c.mutex.Unlock()
		c.sideKick.msgChan <- msg{sig: sigDelete, itemKey: key}
	}
}

func (c *MemCache) Key(r *http.Request) string {
	return r.URL.String()
}

func (c *MemCache) WillAccept(data *[]byte) bool{
	log.Debugf("%d vs %d", len(*data),c.MaxItemSize)
	return len(*data) < c.MaxItemSize
}

func New(maxItemSize int) *MemCache {
	c := MemCache{ MaxItemSize:maxItemSize }
	c.Init()
	return &c
}


