#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
echo "SCRIPT_ROOT: $SCRIPT_ROOT"


#Adjust this to your primary module import path and CRD definition
CURRENT_MODULE_IMPORT_PATH="github.com/devopsext/configurator"
CRD_TYPE_VERSION="rtcfg:v1alpha1"

GENERATED_PACKAGES_IMPORT_PATH="${CURRENT_MODULE_IMPORT_PATH}/pkg/generated"
GENERATED_PACKAGES_TARGET_PATH="${SCRIPT_ROOT}"
APIS_PACKAGE_IMPORT_PATH="${CURRENT_MODULE_IMPORT_PATH}/pkg/apis"


OUTPUT_BASE=$(dirname "${BASH_SOURCE[0]}")
echo "Output base: ${OUTPUT_BASE}" #It is a base directory where to output generated files

GO_HEADER_FILE=${SCRIPT_ROOT}/codegen/boilerplate.go.txt
echo "Go-header-file: ${GO_HEADER_FILE}" #Where boilerplate for code generation stored

bash "${SCRIPT_ROOT}/codegen/generate-groups.sh" "deepcopy,client,informer,lister" \
  "${GENERATED_PACKAGES_IMPORT_PATH}" "${APIS_PACKAGE_IMPORT_PATH}" \
  ${CRD_TYPE_VERSION} \
  --go-header-file "${GO_HEADER_FILE}" \
  --output-base "${OUTPUT_BASE}"

#Move generated code to GENERATED_PACKAGES_TARGET_PATH
if [[ -d ${OUTPUT_BASE}/${CURRENT_MODULE_IMPORT_PATH} ]]; then
  cp -rf ${OUTPUT_BASE}/${CURRENT_MODULE_IMPORT_PATH}/pkg  ${GENERATED_PACKAGES_TARGET_PATH}
  rm -rf ${OUTPUT_BASE}/$(echo "$CURRENT_MODULE_IMPORT_PATH" | cut -d "/" -f1)
fi
