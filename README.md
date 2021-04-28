# Managed By Codefresh ArgoCD installer

Codefresh providing [dashboard](https://codefresh.io/docs/docs/ci-cd-guides/gitops-deployments/) for watching on all activities that happens on argocd side. Codefresh argocd agent important part for check all argocd CRD use watch api and notify codefresh about all changes. 

Like: 
* Application created/removed/updated
* Project created/removed/updated
* Your manifest repo information base on context that you provide to us during installation

In addition this agent do automatic application sync between argocd and codefresh 



## Prerequisites

Make sure that you have

* a [Codefresh account](https://codefresh.io/docs/docs/getting-started/create-a-codefresh-account/) with enabled gitops feature
* a [Codefresh API token](https://codefresh.io/docs/docs/integrations/codefresh-api/#authentication-instructions) that will be used as a secret in the agent
* a [Codefresh CLI](https://codefresh-io.github.io/cli/) that will be used for install agent

## Installation     
 

```sh
codefresh install gitops argocd-agent 
```

## Uninstall     
 

```sh
codefresh uninstall gitops argocd-agent 
```

## Upgrade     

Codefresh will show you indicator inside your [gitops integration](https://g.codefresh.io/account-admin/account-conf/integration/gitops) when you need upgrade your agent

```sh
codefresh upgrade gitops argocd-agent 
```