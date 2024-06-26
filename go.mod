module github.com/bakape/meguca

go 1.22.4

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.4.0

require (
	github.com/ErikDubbelboer/gspt v0.0.0-20190125194910-e68493906b83
	github.com/Masterminds/squirrel v1.1.0
	github.com/abh/geoip v0.0.0-20160510155516-07cea4480daa
	github.com/aquilax/tripcode v1.0.0
	github.com/badoux/goscraper v0.0.0-20181207103713-9b4686c4b62c
	github.com/bakape/captchouli v1.1.5
	github.com/bakape/mnemonics v0.0.0-20170918165711-056d8d325992
	github.com/bakape/thumbnailer v0.0.0-20190424201625-d663001341e2
	github.com/boltdb/bolt v1.3.1
	github.com/chai2010/webp v1.1.0
	github.com/dimfeld/httptreemux v5.0.1+incompatible
	github.com/fsnotify/fsnotify v1.4.7
	github.com/go-playground/log v6.3.0+incompatible
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/websocket v1.4.0
	github.com/lib/pq v1.0.1-0.20190326042056-d6156e141ac6
	github.com/otium/ytdl v0.5.1
	github.com/rakyll/statik v0.1.7
	github.com/sevlyar/go-daemon v0.1.4
	github.com/ulikunitz/xz v0.5.6
	github.com/valyala/quicktemplate v1.7.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

require (
	github.com/PuerkitoBio/goquery v1.5.0 // indirect
	github.com/Sirupsen/logrus v1.4.1 // indirect
	github.com/andybalholm/cascadia v1.0.0 // indirect
	github.com/bakape/boorufetch v1.0.1 // indirect
	github.com/go-playground/ansi v2.1.0+incompatible // indirect
	github.com/go-playground/errors v3.3.0+incompatible // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/julienschmidt/httprouter v1.2.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/mattn/go-sqlite3 v1.10.0 // indirect
	github.com/nwaples/rardecode v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	gitlab.com/nyarla/go-crypt v0.0.0-20160106005555-d9a5dc2b789b // indirect
	golang.org/x/net v0.0.0-20210510120150-4163338589ed // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/bakape/thumbnailer/v2 => github.com/HeyPuter/thumbnailer/v2 v2.0.0-20230828231719-c526241bacef

// replace github.com/bakape/boorufetch => github.com/gummyfrog/remiboorufetch v1.0.2
replace github.com/bakape/boorufetch => /mnt/meguca/miladychan/booru/remiboorufetch

replace github.com/bakape/captchouli => /mnt/meguca/miladychan/booru/captchouli

// github.com/bakape/captchouli v1.1.5
