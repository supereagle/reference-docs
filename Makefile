K8SIOROOT=../../../../../go/src/k8s.io/kubernetes.github.io
K8SROOT=../../../../../go/src/k8s.io/kubernetes

default:
	echo "Support commands:\ncli api copycli copyapi updateapispec"

brodocs:
	docker build . -t pwittrock/brodocs
	docker push pwittrock/brodocs

updateapispec:
	cp $(K8SROOT)/api/openapi-spec/swagger.json gen-apidocs/generators/openapi-spec/swagger.json

# Build kubectl docs
cleancli:
	rm -f main
	rm -rf $(shell pwd)/gen-kubectldocs/generators/includes
	rm -rf $(shell pwd)/gen-kubectldocs/generators/build
	rm -rf $(shell pwd)/gen-kubectldocs/generators/manifest.json

cli: cleancli
	go run gen-kubectldocs/main.go --kubernetes-version v1_6
	docker run -v $(shell pwd)/gen-kubectldocs/generators/includes:/source -v $(shell pwd)/gen-kubectldocs/generators/build:/build -v $(shell pwd)/gen-kubectldocs/generators/:/manifest pwittrock/brodocs

copycli: cli
	rm -rf gen-kubectldocs/generators//build/documents/
	rm -rf gen-kubectldocs/generators//build/runbrodocs.sh
	rm -rf gen-kubectldocs/generators//build/manifest.json
	rm -rf $(K8SIOROOT)/docs/user-guide/kubectl/v1.6/*
	cp -r gen-kubectldocs/generators//build/* $(K8SIOROOT)/docs/user-guide/kubectl/v1.6/

api: cleanapi
	go run gen-apidocs/main.go --config-dir=gen-apidocs/generators
	docker run -v $(shell pwd)/gen-apidocs/generators/includes:/source -v $(shell pwd)/gen-apidocs/generators/build:/build -v $(shell pwd)/gen-apidocs/generators/:/manifest pwittrock/brodocs

# Build api docs
cleanapi:
	rm -f main
	rm -rf $(shell pwd)/gen-apidocs/generators/build
	rm -rf $(shell pwd)/gen-apidocs/generators/includes
	rm -rf $(shell pwd)/gen-apidocs/generators/manifest.json
	rm -rf $(shell pwd)/gen-apidocs/generators/includes/_generated_*

copyapi: api
	rm -rf gen-apidocs/generators/build/documents/
	rm -rf gen-apidocs/generators/build/runbrodocs.sh
	rm -rf gen-apidocs/generators/build/manifest.json
	rm -rf $(K8SIOROOT)/docs/api-reference/v1.6/*
	cp -r gen-apidocs/generators/build/* $(K8SIOROOT)/docs/api-reference/v1.6/

# Build resource docs
resource: cleanapi
	go run gen-apidocs/main.go --build-operations=false
	docker run -v $(shell pwd)/gen-apidocs/generators/includes:/source -v $(shell pwd)/gen-apidocs/generators/build:/build -v $(shell pwd)/gen-apidocs/generators/:/manifest pwittrock/brodocs

copyresource: resource
	rm -rf gen-apidocs/generators/build/documents/
	rm -rf gen-apidocs/generators/build/runbrodocs.sh
	rm -rf gen-apidocs/generators/build/manifest.json
	rm -rf $(K8SIOROOT)/docs/resources-reference/v1.6/*
	cp -r gen-apidocs/generators/build/* $(K8SIOROOT)/docs/resources-reference/v1.6/
