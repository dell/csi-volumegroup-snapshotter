# start server from csi driver integration test folder

CSI_DRIVER=/root/csi-vxflexos

TEST_DIR=test/integration

CSI_ENDPOINT=unix://${CSI_DRIVER}/${TEST_DIR}/unix_sock

# copy modified int server to call tests in csi-extensions
cp ./driver_test.go ${CSI_DRIVER}/${TEST_DIR}/driver_test.go

cd ${CSI_DRIVER}/${TEST_DIR}

rm -f ${CSI_DRIVER}/${TEST_DIR}/unix_sock

export CSI_ENDPOINT=${CSI_ENDPOINT}
export X_CSI_VXFLEXOS_ENABLESNAPSHOTCGDELETE=true

# runs for 5 minutes and stops , adjust sleep in integration_test.go if you need it 
go test driver_test.go -v >int.log &

exit 0
