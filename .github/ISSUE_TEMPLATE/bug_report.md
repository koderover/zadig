---
name: Bug report
about: Tell us about a problem you are experiencing
title: "[bug]"
labels: bug
assignees: jamsman94

---

<!--
You don't need to remove this comment section, it's invisible on the issues page.

## General remarks
* Attention, please fill out this issues form using English only!
-->

**What happened?**
<!--Please provide as much info as possible. Not doing so may result in your bug not being addressed in a timely manner.
For UI bugs please add screenshots or videos that shows the issue.-->

**What did you expect to happen?**
<!--A clear and concise description of what you expected to happen.-->

**How To Reproduce it(as minimally and precisely as possible)**
<!--For example:

Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. See errors
-->

**Install Methods**
<!-- Please set it checked according to the actual situation`- [x] (Script base on K8s)` -->
- [ ] Helm
- [ ] Script base on K8s
- [ ] All in One
- [ ] Offline

**Versions Used**
zadig:
<!--Please provide the version of the zadig you are using. -->
kubernetes: 
<!--Please provide the version of the kubernetes you are using. -->

**Environment**

Cloud Provider:
<!-- For example: Tencent Cloud（TKE）, AliCloud（ACK）, Self-hosting ... -->

Resources:
<!-- For example: CPU/Memory 4 Cores / 8 GB RAM  -->

OS:
<!-- For example: CentOS 7.6,Ubuntu 18.04 LTS -->

**Services Status**
```console
kubectl version
kubectl get po -n `zadig-installed-namespace`
# paste output here
```

If there is abnormal service, please provide service log

```console
kubectl describe pods `abnormal-pod`
kubectl logs --tail=500 `abnormal-pod`
# paste output here
```


