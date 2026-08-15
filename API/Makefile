update :
	"Updating dependencies"
	go get -u

run : feedbridge
	# "changing permissions"
	# sudo chmod u+x feedbridge
	"Running feedbridge"
	./feedbridge

feedbridge : main.go
	go build -o feedbridge main.go 
	echo "Compiled feedbridge"

clean:
	rm -f feedbridge
