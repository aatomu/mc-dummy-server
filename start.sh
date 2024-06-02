screen -r mc_dummy -X stuff "^C\n"
sleep 1s
screen -r mc_dummy -X kill
screen -U -A -md -S mc_dummy
screen -r mc_dummy -X stuff "cd $(dirname $0)/\n"
screen -r mc_dummy -X stuff "while :; do go run . -port=1038; done\n"
