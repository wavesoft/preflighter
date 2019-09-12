package main

//
// The following snippet contains a list of bash functions that will be
// always available to the probe scripts
//
var BashLibrary = `
# Shorthand to 'curl -H <Auth> <DCOS_URL>/'
function cluster_curl() {
  local URL=$1; shift
  curl $* -k -f -L -s -H "Authorization: token=${DCOS_ACS_TOKEN}" ${DCOS_URL}/${URL}
}
function cached_cluster_curl() {
  local URL=$1; shift
  local CACHE_ID=$(echo "${DCOS_URL}|curl|${URL}" | shasum - | awk '{print $1}')
  echo "[curl] Using cache ID: $CACHE_ID" >&2
  local CACHE_FILE="${CACHE_DIR}/${CACHE_ID}"
  if [ ! -f "${CACHE_FILE}" ]; then
    cluster_curl $URL $* > ${CACHE_FILE}
    RET=$?
    if [ $RET != 0 ]; then
      rm ${CACHE_FILE}
      return $RET
    fi
  fi
  cat ${CACHE_FILE}
}

# Perform a bash command on the specified node, making sure only the
# standard output of the command given will be returned
function node_ssh() {
  local NODE_SELECTOR=$1; shift
  dcos node ssh \
    $NODE_SELECTOR \
    --master-proxy \
    --option UserKnownHostsFile=/dev/null \
    --option StrictHostKeyChecking=no \
    --option BatchMode=yes \
    --user=centos \
    "$* 2>&1" | tr '\r' '\n'
  return ${PIPESTATUS[0]}
}
function cached_node_ssh() {
  local CACHE_ID
  local CACHE_FILE
  CACHE_ID=$(echo "${DCOS_URL}|ssh|$*" | shasum - | awk '{print $1}')
  echo "[ssh] Using cache ID: $CACHE_ID" >&2
  CACHE_FILE="${CACHE_DIR}/${CACHE_ID}"
  if [ ! -f "${CACHE_FILE}" ]; then
    node_ssh $* > ${CACHE_FILE}
    RET=$?
    if [ $RET != 0 ]; then
      rm ${CACHE_FILE}
      return $RET
    fi
  fi
  cat ${CACHE_FILE}
}

# Cached call to 'dcos ...'
function cached_dcos() {
  local CACHE_ID=$(echo "${DCOS_URL}|dcos|$*" | shasum - | awk '{print $1}')
  echo "[dcos] Using cache ID: $CACHE_ID" >&2
  local CACHE_FILE="${CACHE_DIR}/${CACHE_ID}"
  if [ ! -f "${CACHE_FILE}" ]; then
    dcos $* > ${CACHE_FILE}
    RET=$?
    if [ $RET != 0 ]; then
      rm ${CACHE_FILE}
      return $RET
    fi
  fi
  cat ${CACHE_FILE}
}

`
