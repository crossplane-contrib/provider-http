# Namespaced HTTP Provider Examples

This directory contains examples for the namespaced version of the HTTP provider resources. These resources use the `http.m.crossplane.io` API group and provide namespace-scoped alternatives to the cluster-scoped resources in the `http.crossplane.io` API group.

## Key Differences from Cluster-scoped Resources

### API Group
- **Cluster-scoped**: `http.crossplane.io`
- **Namespaced**: `http.m.crossplane.io` (follows the `.m.` convention)

### Scope
- **Cluster-scoped**: Resources are available cluster-wide
- **Namespaced**: Resources are confined to a specific namespace

### Provider Configuration Options

#### ProviderConfig (Namespace-scoped)
```yaml
apiVersion: http.m.crossplane.io/v1alpha2
kind: ProviderConfig
metadata:
  name: http-conf-namespaced
  namespace: default  # Confined to this namespace
```

#### ClusterProviderConfig (Cluster-scoped)
```yaml
apiVersion: http.m.crossplane.io/v1alpha2
kind: ClusterProviderConfig
metadata:
  name: http-conf-cluster  # No namespace - cluster-wide
```

## When to Use Each Approach

### Use Cluster-scoped Resources When:
- You need shared configuration across multiple namespaces
- You have cluster-admin privileges
- You want centralized management of HTTP resources
- Resources are shared infrastructure components

### Use Namespaced Resources When:
- You want namespace isolation for security
- Multiple teams/tenants share the same cluster
- You have namespace-level permissions only
- Resources are application-specific

## Examples Included

### Provider Configurations
1. **providerconfig.yaml** - Namespace-scoped provider configuration
2. **clusterproviderconfig.yaml** - Cluster-scoped provider configuration for cross-namespace access

### Request Examples
3. **request.yaml** - Namespaced HTTP request with full CRUD operations using namespaced ProviderConfig
4. **request-with-clusterproviderconfig.yaml** - Namespaced HTTP request using ClusterProviderConfig for cross-namespace access

### DisposableRequest Examples  
5. **disposablerequest.yaml** - Namespaced one-time HTTP request using namespaced ProviderConfig
6. **disposablerequest-jwt.yaml** - Namespaced JWT token acquisition example
7. **disposablerequest-with-clusterproviderconfig.yaml** - Namespaced one-time HTTP request using ClusterProviderConfig

## Usage

1. Apply the provider configuration:
   ```bash
   # For namespace-scoped resources
   kubectl apply -f providerconfig.yaml
   
   # For cluster-scoped cross-namespace access
   kubectl apply -f clusterproviderconfig.yaml
   ```

2. Apply the resource examples:
   ```bash
   kubectl apply -f request.yaml
   kubectl apply -f disposablerequest.yaml
   ```

## Migration from Cluster-scoped Resources

If you're migrating from cluster-scoped resources (`http.crossplane.io`), you'll need to:

1. Update the `apiVersion` from `http.crossplane.io/v1alpha2` to `http.m.crossplane.io/v1alpha2`
2. Add a `namespace` field to the metadata
3. Update the `providerConfigRef` to reference a namespaced ProviderConfig
4. Ensure secrets referenced in `secretInjectionConfigs` are in the same namespace

## Cross-namespace Access

If you need to access secrets or resources in different namespaces, use a `ClusterProviderConfig` instead of a namespace-scoped `ProviderConfig`. The ClusterProviderConfig allows cross-namespace operations while the resource itself remains namespaced.