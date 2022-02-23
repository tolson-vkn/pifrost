#!/bin/bash

# asciinema rec ./demo/demo.cast --overwrite
########################
# include the magic
########################
# Get from https://github.com/paxtonhare/demo-magic
. /usr/local/bin/demo-magic.sh

TYPE_SPEED=40

# hide the evidence
clear

p "Let's start by looking if we can get our DNS record:"
p "\tenv-echgo-lb.tolson.io"

pei "dig env-echgo-lb.tolson.io"

p "Not exactly what we wanted..."

clear

p "Let's deploy pifrost"
pei "kubectl apply -f deployment/00-namespace.yaml; kubectl apply -f deployment/"

sleep 2

pe "kubectl get -f  deployment/"

p "How about some logs?"
pei "stern -n pifrost \".*\""
