module github.com/turutcrane/cefclient-go

go 1.14

require (
	github.com/JamesHovious/w32 v1.1.1-0.20200207125429-4707417e0562
	github.com/turutcrane/cefingo v0.2.10
	github.com/turutcrane/win32api v0.0.0-00010101000000-000000000000
)

replace github.com/turutcrane/cefingo => ../cefingo

replace github.com/turutcrane/win32api => ../win32api

replace github.com/JamesHovious/w32 => ../../JamesHovious/w32
