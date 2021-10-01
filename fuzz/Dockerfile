# Build the manager binary
FROM golang:1.16-buster as builder

RUN apt-get update && apt-get install -y git clang
RUN git clone https://github.com/fluxcd/image-reflector-controller /image-reflector-controller
RUN mkdir /image-reflector-controller/fuzz
COPY fuzz.go /image-reflector-controller/controllers/

RUN cd /image-reflector-controller/controllers \
	&& go get github.com/AdaLogics/go-fuzz-headers \
	&& go get go.uber.org/zap/zapcore@v1.18.1

RUN cd / \
	&& go get -u github.com/mdempsky/go114-fuzz-build \
	&& go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

RUN mkdir /fuzzers
RUN cd /image-reflector-controller/controllers \
	&& go114-fuzz-build -o Fuzz.a -func Fuzz . \
	&& clang -o /fuzzers/Fuzz Fuzz.a -fsanitize=fuzzer

RUN cd /image-reflector-controller/controllers && /fuzzers/Fuzz 
