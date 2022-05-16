package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

var keyPollingDelay time.Duration = 10

var wg = sync.WaitGroup{}

var intervalCh = make(chan struct{})  // Triggers interval stat calculation
var doneCh = make(chan struct{})      // Triggers end of logging
var timeCh = make(chan time.Duration) // Passes elapsed time to calculateStats()

// TimeControls All necessary variables for controlling timed events
type TimeControls struct {
	interval   int
	limit      int
	start      time.Time
	ticker     *time.Ticker
	timer      *time.Timer
	timedMode  bool
	triggerEnd chan struct{}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Typing Meter")
	fmt.Println("---------------------")

	fmt.Print("Enter interval time: ")
	scanner.Scan()
	interval, _ := strconv.Atoi(scanner.Text())

	fmt.Print("Enter time limit: ")
	scanner.Scan()
	limit, _ := strconv.Atoi(scanner.Text())

	wg.Add(2)

	timeControls := TimeControls{
		interval:   interval,
		limit:      limit,
		start:      time.Now(),
		ticker:     time.NewTicker(time.Duration(interval) * time.Second),
		timer:      time.NewTimer(time.Duration(limit) * time.Second),
		timedMode:  true,
		triggerEnd: make(chan struct{}),
	}

	fmt.Println("\nStarting meter")
	fmt.Println("---------------------")

	go readInput(&timeControls)
	go timeController(&timeControls)

	wg.Wait()
}

func readInput(timeControls *TimeControls) {
	var keys []byte
	var index int

	var lastKey int
	var activeKey int

loop:
	for {
		activeKey = readKey() // Gets value of pressed key

		if activeKey != 0 { // If a key is pressed
			if activeKey != lastKey { // If not holding down same key
				lastKey = activeKey
				keys = append(keys, byte(activeKey))

				startIndex := 0
				if len(keys) >= 5 {
					startIndex = len(keys) - 5
				}

				input := string(keys[startIndex:]) // Converting last 5 characters to one string

				switch {
				case strings.Contains(input, "START"):
					fmt.Println("\nCommand received: START")
					fmt.Println("Switching from timed mode to manual mode")
					fmt.Println("Type END to stop meter")

					// Resetting all variables to initial values
					timeControls.start = time.Now()
					timeControls.ticker.Reset(time.Duration(timeControls.interval) * time.Second)
					timeControls.timer.Stop()
					timeControls.timedMode = false

					keys = nil
					index = 0
				case strings.Contains(input, "END"):
					if !timeControls.timedMode {
						fmt.Println("\nCommand received: END")
						timeControls.triggerEnd <- struct{}{} // Sending trigger to timeController()
					}
				}
			}
		} else {
			lastKey = 0
		}

		select {
		case <-intervalCh:
			wg.Add(1)
			fmt.Println("\nInterval statistics:")
			go calculateStats(keys[index:], len(keys), false) // Sending slice with only the keys pressed during last interval

			index = len(keys) // Saving new index for next interval
		case <-doneCh:
			close(doneCh)
			close(intervalCh)

			break loop
		default:
			time.Sleep(keyPollingDelay * time.Millisecond)
		}
	}

	wg.Add(1)
	fmt.Println("\nSession complete, overall statistics:")
	go calculateStats(keys, len(keys), true)

	wg.Done()
}

func readKey() (activeKey int) {
	for i := 0; i < 256; i++ {
		keyState, _, _ := procGetAsyncKeyState.Call(uintptr(i)) // Checking keystate for all keys in range (0, 255)

		if keyState&(1<<15) != 0 && !(i < 0x2F && i != 0x20) && (i < 160 || i > 165) && (i < 91 || i > 93) { // If key is pressed and satisfies conditions
			activeKey = i
			break
		} else {
			activeKey = 0
		}
	}

	return activeKey
}

func calculateStats(keys []byte, totalKeyCount int, end bool) {
	timeElapsed := <-timeCh // Receiving time since session start
	if end {
		fmt.Printf("Total time elapsed: %v\n", timeElapsed)
		close(timeCh)
	}

	total := len(keys)
	speed := float64(totalKeyCount) / timeElapsed.Seconds()

	// Iterating over keys and increasing appropriate value in map
	popularityMap := make(map[string]int)
	for _, key := range keys {
		popularityMap[string(key)]++
	}

	// Function takes map as parameter and returns slice of structs with Key and Value properties
	sorted := sortMapByValue(popularityMap)

	fmt.Printf("Keys pressed: %v\n", total)
	fmt.Printf("Typing speed: %.2f\n", speed)

	fmt.Println("Most pressed keys:")
	for i := 0; i < 3; i++ {
		if len(sorted) > i {
			fmt.Printf("%v. \"%v\": %v\n", i+1, sorted[i].Key, sorted[i].Value)
		}
	}

	wg.Done()
}

func timeController(timeControls *TimeControls) {
loop:
	for {
		select {
		case <-timeControls.triggerEnd: // Waits for manual trigger of end
			timeControls.ticker.Stop()
			doneCh <- struct{}{}
			timeCh <- time.Since(timeControls.start)

			break loop
		case <-timeControls.timer.C: // Waits for timed trigger of end
			timeControls.ticker.Stop()
			doneCh <- struct{}{}
			timeCh <- time.Since(timeControls.start)

			break loop
		case <-timeControls.ticker.C: // Waits for interval trigger
			intervalCh <- struct{}{}
			timeCh <- time.Since(timeControls.start)
		}
	}

	wg.Done()
}

// Pair A data structure to hold a key/value pair.
type Pair struct {
	Key   string
	Value int
}

// PairList A slice of Pairs that implements sort.Interface to sort by Value.
type PairList []Pair

func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }

// A function to turn a map into a PairList, then sort and return it.
func sortMapByValue(m map[string]int) PairList {
	p := make(PairList, len(m))
	i := 0
	for k, v := range m {
		p[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(p))
	return p
}
