module github.com/mkmccarty/TokenTimeBoostBot

go 1.25.0

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/mkmccarty/TokenTimeBoostBot/src/boost v0.0.0-20250815152534-58911d3a4bc5
	github.com/mkmccarty/TokenTimeBoostBot/src/bottools v0.0.0-20250815152534-58911d3a4bc5
	github.com/mkmccarty/TokenTimeBoostBot/src/config v0.0.0-20250815152534-58911d3a4bc5
	github.com/mkmccarty/TokenTimeBoostBot/src/events v0.0.0-20250815152534-58911d3a4bc5
	github.com/mkmccarty/TokenTimeBoostBot/src/version v0.0.0-20250815152534-58911d3a4bc5
)

require (
	github.com/divan/num2words v1.0.1 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/moby/moby v28.3.3+incompatible // indirect
	github.com/peterbourgon/diskv/v3 v3.0.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/image v0.30.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
)

replace github.com/mkmccarty/TokenTimeBoostBot/src/boost => ./src/boost

replace github.com/mkmccarty/TokenTimeBoostBot/src/bottools => ./src/bottools

replace github.com/mkmccarty/TokenTimeBoostBot/src/config => ./src/config

replace github.com/mkmccarty/TokenTimeBoostBot/src/events => ./src/events

replace github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate => ./src/farmerstate

replace github.com/mkmccarty/TokenTimeBoostBot/src/notok => ./src/notok

replace github.com/mkmccarty/TokenTimeBoostBot/src/tasks => ./src/tasks

replace github.com/mkmccarty/TokenTimeBoostBot/src/track => ./src/track

replace github.com/mkmccarty/TokenTimeBoostBot/src/version => ./src/version
