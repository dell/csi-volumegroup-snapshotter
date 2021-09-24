export CSI_ENDPOINT=/tmp/unix_sock
export X_CSI_VXFLEXOS_ENABLESNAPSHOTCGDELETE=false
go test -coverprofile=c.linux.out -v *.go
