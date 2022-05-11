# Test for Kubebuilder

- If you want to know if the operation is normal, you can use port-forword to map the cluster port to the host
```shell
    kubectl port-forward -n dev svc/elasticweb-sample 30003:8080 > pf30003.out &
```
- Check port usage
```shell
    lsof -i:30003
```

## USE

### Deploy CRD and Verify Api

1. make install
2. kubectl api-versions|grep elasticweb

### Run Controller Local
1. make run
2. kubectl apply -f config/samples/webapp_v1_elasticweb.yaml
3. kubectl get elasticweb -n dev // see CR
4. kubectl get service -n dev
5. kubectl get deployment -n dev
6. kubectl get pod -n dev

### Change Single Pod QPS
```shell
    kubectl patch elasticweb elasticweb-sample \
    -n dev \
    --type merge \
    --patch "$(cat config/samples/update_single_pod_qps.yaml)"
```

### Change Total Pod QPS
```shell
    kubectl patch elasticweb elasticweb-sample \
    -n dev \
    --type merge \
    --patch "$(cat config/samples/update_total_qps.yaml)"
```