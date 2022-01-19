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

cd "${GO_SRC}"

# Move fuzzer to their respective directories. 
# This removes dependency noises from the modules' go.mod and go.sum files.
cp "${PROJECT_PATH}/tests/fuzz/fuzz_controllers.go" "${PROJECT_PATH}/controllers/"

# Some private functions within suite_test.go are extremly useful for testing.
# Instead of duplicating them here, or refactoring them away, this simply renames
# the file to make it available to "non-testing code".
# This is a temporary fix, which will cease once the implementation is migrated to
# the built-in fuzz support in golang 1.18.
cp "${PROJECT_PATH}/controllers/registry_test.go" "${PROJECT_PATH}/controllers/fuzzer_helper.go"

# compile fuzz tests for the runtime module
pushd "${PROJECT_PATH}"

# Setup files to be embedded into controllers_fuzzer.go's testFiles variable.
mkdir -p controllers/testdata/crd
cp config/crd/bases/*.yaml controllers/testdata/crd

go mod tidy
compile_go_fuzzer "${PROJECT_PATH}/controllers/" FuzzImageRepositoryController fuzz_imagerepositorycontroller

popd
