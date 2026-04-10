3 launch blockers (C1–C3):
- Client can self-deposit any amount → remove /wallets/deposit from client routes
- Withdraw double-spend race → use AtomicDeduct
- CreateExchange race + broken rollback → needs DB transaction or atomic ops

7 logic bugs (L1–L7):
- CancelExchange always fails (exchanges born as completed)
- ConfirmPaymentReceived on expired orders corrupts locked balance
- OTC GetOrder side-effects before auth check
- AcceptOffer audit log records wrong role
- Nil panic in CreateExchange email goroutine
- Wallet creation errors silently dropped at registration
- Email check swallows DB errors at registration

3 dead code items (D1–D3):
- SetProfile/UpdateProfile are identical
- UserService.GetUser unreachable
- UserService.UpdateUser unreachable