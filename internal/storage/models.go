package storage 
import "time"

type Message struct {
	ID 			int64 		`json:"id"`
	RoomID 		string 		`json:"room_id"`
	Username 	string 		`json:"username"`
	Content 	string 		`json:"content"`
	CreatedAt 	time.Time 	`json:"created_at"`
}

