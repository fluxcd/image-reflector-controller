#!/usr/bin/env bash

# Copyright 2022 The Flux authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euxo pipefail

GOPATH="${GOPATH:-/root/go}"
GO_SRC="${GOPATH}/src"
PROJECT_PATH="github.com/fluxcd/image-reflector-controller"

pushd "${GO_SRC}/${PROJECT_PATH}"

# The go.mod in tests/fuzz only exists to avoid dependency pollution
# in the main module.
rm tests/fuzz/go.mod

go get -d github.com/AdaLogics/go-fuzz-headers

# Setup files to be embedded into controllers_fuzzer.go's testFiles variable.
mkdir -p tests/fuzz/testdata/crd
cp config/crd/bases/*.yaml tests/fuzz/testdata/crd

compile_go_fuzzer "${PROJECT_PATH}/tests/fuzz/" FuzzImageRepositoryController fuzz_imagerepositorycontroller

popd
