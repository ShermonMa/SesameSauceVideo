cd /root/sesame/Sesame-sauce/backend
pkill sesame-test
git pull
cd /root/sesame/Sesame-sauce/backend && go build -o sesame-test cmd/api/main.go
cd /root/sesame/Sesame-sauce/backend && nohup ./sesame-test > test.log 2>&1 &