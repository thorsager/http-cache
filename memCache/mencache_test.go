package memCache

import (
	"testing"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"reflect"
)

var c MemCache

func init() {
	log.SetLevel(log.DebugLevel)
}

func setupFunc() func() {
	c = MemCache{MaxItemSize:100}
	c.Init()
	return func() {
		defer c.Close()
	}
}

func TestMemCache_Get2(t *testing.T) {
	c := MemCache{}
	defer c.Close()
	_, ok :=c.Get("anything")
	if ok {
		t.Error("Should not return OK on Get from un-initialized Cache")
	}
}
func TestMemCache_Put2(t *testing.T) {
	c := MemCache{}
	defer c.Close()
	ok := c.Put("anything",make([]byte,10))
	if ok {
		t.Error("Should not return OK on Put to un-initialized Cache")
	}
}
func TestMemCache_Delete(t *testing.T) {
	c := MemCache{}
	c.Delete("anything")
	defer c.Close()
}

func TestMemCache_Init(t *testing.T) {
	c := MemCache{}
	c.Init()
	defer c.Close()
}

func TestMemCache_WillAcceptFail(t *testing.T) {
	tearDown := setupFunc()
	defer tearDown()

	d := make([]byte, 200)

	ok := c.WillAccept(&d)
	t.Logf("OK=%t",ok)
	if ok {
		t.Errorf("Should not Accept OversizedItems (%d vs %d)",c.MaxItemSize,len(d))
	}
}
func TestMemCache_WillAcceptPass(t *testing.T) {
	tearDown := setupFunc()
	defer tearDown()
	d := make([]byte, 50)

	ok := c.WillAccept(&d)
	t.Logf("OK=%t",ok)
	if !ok {
		t.Errorf("Should Accept item of size (%d vs %d)",c.MaxItemSize,len(d))
	}
}

func TestMemCache_PutToBig(t *testing.T) {
	tearDown := setupFunc()
	defer tearDown()
	d := make([]byte, 200)

	if c.Put("test",d) {
		t.Errorf("Cache Should not allow OversizedItems (%d vs %d)",c.MaxItemSize,len(d))
	}
}

func TestMemCache_Put(t *testing.T) {
	tearDown := setupFunc()
	defer tearDown()

	d := make([]byte, 50)
	rand.Read(d)

	if ok:=c.Put("foo",d); !ok {
		t.Error("Put failed!")
	}

	if c.Size() != 1 {
		t.Error("Size does not fit one successful Put")
	}
}

func TestMemCache_Get(t *testing.T) {
	const key = "testing"
	tearDown := setupFunc()
	defer tearDown()

	d := make([]byte, 50)
	rand.Read(d)


	if ok:=c.Put(key,d);!ok {
		t.Error("Cache should accept data!")
	}

	data, ok := c.Get(key)
	if !ok {
		t.Errorf("Item %s should be in cache",key)
	} else {
		if ! reflect.DeepEqual(data,d) {
			t.Errorf("Data _GOT_ does not match data _PUT_ %v vs %v",data,d)
		}
	}
}
