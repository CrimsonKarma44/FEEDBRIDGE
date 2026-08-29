#!/usr/bin/env bash
# Create the Always Free GCE e2-micro used by FEEDBRIDGE.
# Prerequisites: gcloud SDK, a GCP project with Compute Engine API enabled,
# and billing (Always Free still requires a billing account).
#
# Usage:
#   ./deploy/gcp/create-vm.sh
# Optional env:
#   GCP_PROJECT  GCP_ZONE=us-central1-a  GCP_INSTANCE=feedbridge
set -euo pipefail

PROJECT="${GCP_PROJECT:-$(gcloud config get-value project 2>/dev/null)}"
ZONE="${GCP_ZONE:-us-central1-a}"
NAME="${GCP_INSTANCE:-feedbridge}"

if [[ -z "$PROJECT" || "$PROJECT" == "(unset)" ]]; then
    echo "set a project: gcloud config set project YOUR_PROJECT_ID" >&2
    exit 1
fi

echo "==> project=${PROJECT} zone=${ZONE} instance=${NAME}"

if ! gcloud compute instances describe "$NAME" --zone="$ZONE" --project="$PROJECT" >/dev/null 2>&1; then
    echo "==> creating e2-micro (Always Free: us-central1 / us-west1 / us-east1 only)"
    gcloud compute instances create "$NAME" \
        --project="$PROJECT" \
        --zone="$ZONE" \
        --machine-type=e2-micro \
        --image-family=ubuntu-2404-lts-amd64 \
        --image-project=ubuntu-os-cloud \
        --boot-disk-size=30GB \
        --boot-disk-type=pd-standard \
        --tags=feedbridge
else
    echo "==> instance ${NAME} already exists"
fi

if ! gcloud compute firewall-rules describe feedbridge-ssh --project="$PROJECT" >/dev/null 2>&1; then
    echo "==> firewall: tcp:22 to tag feedbridge"
    gcloud compute firewall-rules create feedbridge-ssh \
        --project="$PROJECT" \
        --allow=tcp:22 \
        --target-tags=feedbridge \
        --description="SSH only for the FEEDBRIDGE VM"
else
    echo "==> firewall rule feedbridge-ssh already exists"
fi

IP="$(gcloud compute instances describe "$NAME" --zone="$ZONE" --project="$PROJECT" \
    --format='get(networkInterfaces[0].accessConfigs[0].natIP)')"

cat <<EOF

VM is up: ${NAME}  ${IP}

Next (from this machine):
  gcloud compute ssh ${NAME} --zone=${ZONE} --project=${PROJECT}

On the VM:
  # copy deploy/ over, then:
  cd ~/deploy && sudo ./setup-server.sh

From this machine, after env files are filled:
  make -C API deploy HOST=\$(whoami)@${IP}
  make -C bot deploy HOST=\$(whoami)@${IP}

Do not compile on the VM. Do not open 50051, 5432, or 6379.
EOF
