Hey there,

Good work on writing this down. The context does not make the actual requirements for the solution very clear to me.

As I understood it:

"The main problem we want to solve is to have a process or system that ensures that only one person has access to a system's/device's break-glass account at a given time."

Is that correct? Can we add a concrete problem that we want to solve?

It's not clear to me how the "plugin" is receiving the secret. Is it receiving the secret with an AppCredential or with the user-bound OIDC token?

--- 
I can not follow the separation of concerns argumentation her;, it is totally broken from my perspective:

With this solution, we are mixing the access control: "Who can access a secret at a given time" into an additional system (Netbox) unless you use the user's personal OIDC token which would be ok from my perspective.

If you take the personal token, though, you will have the problem that any permitted person will be able to access the secret as well on Vault directly, and we did not solve the problem then.

I think in general it's a great idea to have a check-in/check-out process for break-glass accounts to solve the problem, but I'm not convinced that the integration in Netbox as described here is the best approach to it.

I may miss some context




I 
