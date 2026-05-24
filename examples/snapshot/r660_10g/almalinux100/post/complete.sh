#!/bin/sh
# Notify metal-install that installation is complete for 02-00-00-00-00-00.
#
# Failure here is acceptable: the install server will simply think
# the node is still pending and re-issue the install on next PXE.
# A re-install immediately after a successful install is idempotent.

curl -X DELETE http://192.0.2.1/nodes/02-00-00-00-00-00 || true
