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

## Notes
- If you don't want to enable the webhook function at startup, you can use the following command to run

```shell
    make run ENABLE_WEBHOOKS=false
```

- If you want to enable the webhook function at startup, you can use the following command to run
```shell
    // 1. Deploying the cert manager
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
    
    // 2. make run
    make run
```

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

### Delete CR

```shell
    kubectl delete elasticweb elasticweb-sample -n dev
```

### Build Docker Image
```shell
    make docker-build docker-push IMG=<your.repo>/elasticweb:0.1
```

### Deploy Image Local Cluster
```shell
    make deploy IMG=<your.repo>/elasticweb:0.1  
```
```shell
    kubectl apply -f config/samples/webapp_v1_elasticweb.yaml
```
- get controller log
```shell
    kubectl logs -f elasticweb-operator-controller-manager-859c755c64-7h5d8 -c manager -n elasticweb-operator-system
```

### Add Default Verify
```shell
    kubectl apply -f config/samples/origin_srd_without_total.yaml --validate=false
```
```shell
    kubectl describe elasticweb elasticweb-sample -n dev // see describe of CR
```

### Add Validator
```shell
    kubectl patch elasticweb elasticweb-sample -n dev --type merge --patch "$(cat config/samples/update_single_pod_max_verify.yaml)" 
```
```shell
    kubectl describe elasticweb elasticweb-sample -n dev // see describe of CR
```