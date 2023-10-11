package bookmark

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func Index(bm Bookmark) {
	rand.Seed(time.Now().UnixNano())
	randomStr := fmt.Sprint(rand.Intn(1000000) + 1)
	bm.Text = "random" + randomStr
	bmDb := BmDb
	_, err := bmDb.Update(bm.URL, bm)
	if err != nil {
		log.Fatal("Update error:", err)
		return
	}
}
