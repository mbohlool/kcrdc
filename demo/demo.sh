step_wait() {
    read -n 1 -s -r
}
my_echo() {
COLOR='\033[0;36m'
NC='\033[0m'
echo -e "${COLOR}$@${NC}"
}

my_curl() {
  curl $@ 2>/dev/null | python -m json.tool  
}

run_cmd() {
COLOR='\033[0;32m'
NC='\033[0m'
echo -e "${COLOR}$ $@${NC}"
$@
}

kubectl proxy > /dev/null &
sleep 1
clear
my_echo "Hi, on each step, press a key to continue..."
step_wait
my_echo "Let's create a CRD which calls into non-existing webhook and has schemas defined for both versions"
step_wait
run_cmd kubectl create -f crd.yaml
step_wait
my_echo "Now let's create a CR in v1 (storage is v1, so no conversion necessary)"
step_wait
run_cmd kubectl create -f cr1.yaml 
step_wait
my_echo "Getting it in v1 should succeed:"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v1/namespaces/default/foos/foo-instance-1
step_wait
my_echo "Getting it in v2 should fail:"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v2/namespaces/default/foos/foo-instance-1
step_wait
my_echo "Creating a cr that is in v2 should fail too:"
step_wait
run_cmd kubectl create -f cr2.yaml 
step_wait
my_echo "Now lets create our webhook:"
step_wait
my_echo "We need some rbac roles to give our webhook access to CRDs in the cluster"
step_wait
run_cmd kubectl create -f rbac.yaml
step_wait
my_echo "Now we create the webhook itself which is a secret, deployment, and service"
step_wait
run_cmd kubectl create -f webhook.yaml
step_wait
my_echo "There should be one pod running our webhook"
step_wait
run_cmd kubectl get pods
step_wait
my_echo "We can check the log of the pot to make sure it is working (skipping this...)"
step_wait
# kubectl log ...
my_echo "Now getting first CR in v2 should succeed:"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v2/namespaces/default/foos/foo-instance-1
step_wait
my_echo "And creating a cr that is in v2 should succeed too:"
step_wait
run_cmd kubectl create -f cr2.yaml 
step_wait
my_echo "Second CR in v1:"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v1/namespaces/default/foos/foo-instance-2
step_wait
my_echo "Second CR in v2:"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v2/namespaces/default/foos/foo-instance-2
step_wait
my_echo "it's fun, lets create another CR:"
step_wait
run_cmd kubectl create -f cr3.yaml
step_wait
my_echo "Now let's list everything in v1 (no conversion should be necessary):"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v1/namespaces/default/foos
step_wait
my_echo "and now the list in v2:"
step_wait
run_cmd my_curl localhost:8001/apis/stable.example.com/v2/namespaces/default/foos
step_wait
run_cmd ./cleanup.sh
my_echo "Bye."
