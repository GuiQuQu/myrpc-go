package myrpc

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

type Foo int

type Args struct {
	Num1, Num2 int
}

func (f Foo) Sum(args *Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(len(s.method) == 1, "wrong service method, expect 1, but got %d", len(s.method))
	mType := s.method["Sum"]
	_assert(mType != nil, "wrong service method, expect Sum, but got %s", mType.method.Name)
}

func TestMethodType_Call(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	methodType := s.method["Sum"]
	argv := methodType.newArgv()
	replyv := methodType.newReplyv()
	if argv.Kind() == reflect.Ptr {
		argv.Elem().Set(reflect.ValueOf(Args{Num1: 1, Num2: 2}))
	} else {
		argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 2}))
	}
	err := s.call(methodType, argv, replyv)
	_assert(err == nil, "failed to call Foo.Sum, err: %v", err)
	_assert(replyv.Elem().Int() == 3, "failed to call Foo.Sum, expect 3, but got %d", replyv.Elem().Int())
}

func TestIsCloseChannelNecessary_on_equal(t *testing.T) {
	fmt.Println("NumGoroutine:", runtime.NumGoroutine())
	ich := make(chan int)

	// sender
	go func() {
		for i := 0; i < 3; i++ {
			ich <- i
		}
	}()

	// receiver
	go func() {
		for i := 0; i < 3; i++ {
			fmt.Println(<-ich)
		}
	}()

	time.Sleep(time.Second)
	fmt.Println("NumGoroutine:", runtime.NumGoroutine())

	// Output:
	// NumGoroutine: 2
	// 0
	// 1
	// 2
	// NumGoroutine: 2
}

func TestTaskControl(t *testing.T) {
	dataChan := make(chan int, 10)
	taskNum := 3
	wg := sync.WaitGroup{}
	wg.Add(taskNum)
	t.Logf("NumGoroutine: %d", runtime.NumGoroutine())
	// start goroutine
	for i := 0; i < taskNum; i++ {
		go func(taskNo int) {
			defer wg.Done()
			for {
				// 使用数据通道close来表示任务结束
				select {
				case v, ok := <-dataChan:
					if !ok {
						t.Logf("Task %d notify to stop\n", taskNo)
						return
					}
					fmt.Printf("Task %d get data %d\n", taskNo, v)
				}
			}
		}(i)
	}
	// after 3s, close dataChan
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Millisecond * 20)
			dataChan <- i

		}
		time.Sleep(time.Second * 3)
		close(dataChan)
	}()

	wg.Wait()
	t.Logf("NumGoroutine: %d", runtime.NumGoroutine())

}


