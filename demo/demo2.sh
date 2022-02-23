#!/bin/bash

# asciinema rec ./demo/demo.cast --overwrite
########################
# include the magic
########################
# Get from https://github.com/paxtonhare/demo-magic
. /usr/local/bin/demo-magic.sh

TYPE_SPEED=40
p "Let's now deploy a loadbalancer service"
pei "kubectl apply -f examples/lb-service.yaml"

p "From the logs above, we can see our DNS entry was added"
p "Let's try the dig again"
pei "dig +noall +answer env-echgo-lb.tolson.io"
p "What about an ingress object?"
clear

pei "dig +noall +answer env-echgo.example.com"
p "Bummer let's fix that"
pei "kubectl apply -f examples/ingress.yaml"
pei "dig +noall +answer env-echgo.example.com"
p "That's all folks..."

