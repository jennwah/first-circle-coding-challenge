# First Circle Coding Challenge

1. The solution is coded in `Go 1.26.1` with clean architecture codebase approach (service, repository, models etc.) together with Go's interface. 
2. Database chosen for this solution is PostgreSQL v17, which can be spinned up locally via Docker with `make db-up`, and initial migrations with `make migrate-up`. 
3. Test the main program with `make run` which runs the account creations, deposits, withdrawals, transfers and transactions functions to demonstrate the functionality.
4. Integration tests are written to test the flows of functions involving databases and covering several scenarios which are crucial for bank transactions context.


## Use of LLM models to complete this challenge

To simulate the real world daily SDE workflow, a coding agent tool (OpenCode) with LLM models (Deepseek Flash v4.1 Flash 1017) is used to complete the challenge together with my prompts. Here's my first initial prompt to kickstart this task:

### Prompt

```bash
Read the @questions.md to first understand the requirements. Then use Go (locally 1.26.1) as the programming language for this task. For database, we will use PostgreSQL 17 with local docker setup (using docker compose) and Makefile for setting up database locally, running init.sql migrations for initial db schema (put it under /migrations directory) and running the main program (make run). 

For the solution, use clean architectue approach to structure the codebase with service/ , repository/ , models/ directory together with Go interface. Use sqlx for db library and shopspring decimal package for all numbers for financials calculations. Use NUMERIC(20, 2) in postgresql for the balances and amount columns. When coding, do not use any COMMENTS , COMMENTS in code are forbidden. Make sure the code is Go idiomatic and clean code.

For the schema design, I suppose we can have two tables, one for accounts table and the other for transactions table, where accounts record user id and balance (use db check to ensure balance >= 0), and transactions table having type columns for deposits, withdrawals, transfer_in, transfer_out, amount column. Create necessary indexes for frequent access patterns.

For tests, write integration tests which tests the flow of service functions which include database interactions end-to-end, with spinned up local test database container, make sure each integration tests database state are reset for correctness testing. Tests for scenarios such as deposits, withdrawals , withdrawals can never overdrew balance, concurrent transfers never cause database deadlock , validations and etc.

Since we are using postgresql as our data store, for the functions on deposits, we can open a db transaction for update account balance and insert a transaction record to guarantee atomic transaction on db. For withdrawals, we will need to hold a row level lock on account balance to check balance, then update +insert transactions accordingly. For transfers, we will need to hold row level locks on both accounts sorted by their id to avoid deadlock scenarios, then check balances, and then perform the balances update and transactions record insert accordingly. Correct me if im wrong on this.

Here's my proposal, propose to me if your implementation, and correct me if im wrong on my proposal. Let me review your proposal before you implement. 
```

Of course, here I demonstrate the initial prompt to kickstart this task together with AI coding agents. In real world daily SDE process, we will have these codebase design principles, no-comments rules, Golang best practices, idiomatic, some preference choices etc written in some markdown files for our coding agents to follow as default rules. Then for any particular task, the workflow is always:

Plan -> Discuss -> Implementation -> Review -> Merge, and then we repeat the workflow concurrently with more agents. 

The final solution of this coding challenge is of course achieved after multiple rounds of code reviews and back-and-forth prompts before I think this is good enough solution.
