# Test for Kubebuilder

- If you want to know if the operation is normal, you can use port-forword to map the cluster port to the host
```shell
    kubectl port-forward -n dev svc/elasticweb-sample 30003:8080 > pf30003.out &
```
- Check port usage
```shell
    lsof -i:30003
```