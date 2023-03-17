# Features

- Full* cluster functionality at test time
  - *YMMV for full cluster features
    - f. e. storage snapshot controllers are not provided by K3s
- clean up containers during start-up failure
  - nobody likes to clean up after other tests ;)
- expose `kubeconfig` to test developer
  - :note: do you want to debug containers? It does not have to be containers :note:
- apply kubernetes resources at cluster start-up time
  - simplify repeated tasks
- kubectl usage
  - do you want to have resources? Because that's how you get resources
- Enable external access to cluster pods
  - Loadbalancer/ingress testing
  - port forward
- generate cluster identifiers automatically
- allow user to choose a custom namespace