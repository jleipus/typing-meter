package main

import (
	"flag"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"typingMeter/mapsort"
	"typingMeter/timectrl"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

// Delay in MS between key reads
const keyPollingDelay time.Duration = 10

var wg = sync.WaitGroup{}

func main() {
	interval := flag.Int("interval", 5, "time in seconds between intervals")
	limit := flag.Int("limit", 20, "time limit in seconds")
	flag.Parse()

	fmt.Println("Typing Meter")
	fmt.Println("---------------------")

	wg.Add(3)

	// Creating new TimeController with a ticker and timer
	tc := timectrl.NewTimeController(timectrl.WithTicker(*interval), timectrl.WithTimer(*limit))

	go tc.RunTicker(&wg)
	go tc.RunTimer(&wg)
	go readInput(tc, *interval)

	wg.Wait()
}

func readInput(tc *timectrl.TimeController, interval int) {
	var keys []byte // Byte slice for all keypresses
	var index int   // Index of the last keypress of the previous interval

	var lastKey int
	var activeKey int

	isFinished := false
	for !isFinished {
		activeKey = readKey() // Gets value of pressed key

		if activeKey != 0 { // If a key is pressed
			if activeKey != lastKey { // If not holding down same key
				lastKey = activeKey
				keys = append(keys, byte(activeKey))

				// If less than 5 keys have been logged, set startIndex to 0
				startIndex := func(len int) int {
					if len-5 < 0 {
						return 0
					}
					return len - 5
				}(len(keys))

				input := string(keys[startIndex:]) // Convert last 5(or less if len(keys) < 5) bytes into string
				switch {
				case strings.Contains(input, "START"):
					fmt.Println("\nCommand received: START")
					fmt.Println("Switching from timed mode to manual mode")
					fmt.Println("Type END to stop meter")

					tc.StopAll()                                                   // Stops the timer and ticker
					tc = timectrl.NewTimeController(timectrl.WithTicker(interval)) // Creating new TimeController with only a ticker

					wg.Add(1)
					go tc.RunTicker(&wg)

					keys = nil
					index = 0
				case strings.Contains(input, "END"):
					if !tc.TimedMode {
						fmt.Println("\nCommand received: END")
						tc.StopAll()      // Since timer should not be running, stopping only ticker
						isFinished = true // Setting loop finishing condition
					}
				}
			}
		} else {
			lastKey = 0 // Prevents holding down a key from being counted as keypress every cycle
		}

		select {
		case <-tc.IntervalCh: // Received from TimeController.ticker
			wg.Add(1)
			fmt.Println("\nInterval statistics:")
			k := keys[index:]
			go calculateStats(&k, len(keys), tc) // Passing only the keys from last interval

			index = len(keys)
		case <-tc.DoneCh: // Received from TimeController.timer
			tc.StopAll()
			isFinished = true
		default:
			time.Sleep(keyPollingDelay * time.Millisecond)
		}
	}

	wg.Add(1)
	fmt.Println("\nSession complete, overall statistics:")
	go calculateStats(&keys, len(keys), tc)

	wg.Done()
}

func readKey() (activeKey int) {
	for i := 0; i < 0xFF; i++ {
		keyState, _, _ := procGetAsyncKeyState.Call(uintptr(i)) // Checking keystate for all keys in range (0, 255)

		// From documentation
		// `if the least significant bit is set, the key was pressed after the previous call to GetAsyncKeyState.`
		// https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getasynckeystate
		// if keyState&(1<<15) != 0 && !(i < 0x2F && i != 0x20) && (i < 160 || i > 165) && (i < 91 || i > 93) {
		if keyState&(1<<15) != 0 {
			activeKey = i
			break
		} else {
			activeKey = 0
		}
	}

	return activeKey
}

func calculateStats(keys *[]byte, totalKeyCount int, tc *timectrl.TimeController) {
	timeElapsed := tc.TimePassed() // Receiving time since session start

	total := len(*keys)
	speed := float64(totalKeyCount) / timeElapsed.Seconds()

	popularityMap := sliceToMap(*keys)              // Map of all keys pressed and times they were pressed
	sorted := mapsort.SortMapByValue(popularityMap) // Sorted map returned as PairList

	fmt.Printf("Keys pressed: %v\n", total)
	fmt.Printf("Typing speed: %.2f\n", speed)

	fmt.Println("Most pressed keys:")
	for i := 0; i < 3; i++ {
		if i < sorted.Len() {
			fmt.Printf("%v. \"%v\": %v\n", i+1, sorted[i].Key, sorted[i].Value)
		}
	}

	wg.Done()
}

// Takes slice of bytes and returns a map with all unique values and times
// they appear in the slice
func sliceToMap(s []byte) map[string]int {
	ret := make(map[string]int)
	for _, key := range s {
		ret[string(key)]++
	}

	return ret
}
