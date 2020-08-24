# ZEO.ORG - Timeout Increaser

This project simply increases AWS API Gateway's timeout limit.

## Why

As you know, AWS API Gateway has a hard limit for timeout; 29 seconds. In today's world, it is normal to expect a service to return in less than 29 seconds. But unfortunately, in the real world, not every situations fit this. There may be situations where we need to keep the client waiting for a long time. That's why we made this structure.

## How

It works simply as follows; when there is an incoming request, the Proxy gives a RequestID to this request and adds a row to DynamoDB as a PENDING status. Then invokes Talker as async.

After that, it checks the status in DynamoDB every 3 seconds, 8 tries at most. If the status becomes CREATED during this process, the result returns to the user after deleting the related row from the database. If the status doesn't become CREATED in the process, which is exactly the situation we want to overcome, returns a 303 response with RequestID to get user one more time.

When the client arrives with a RequestID, the DB is checked again, this process continues until the Status becomes CREATED.

## Structure

**Proxy:** Handles requests from the API Gateway, invokes the Talker as async, returns result by checking DynamoDB.  
**Talker:** Invokes the Main as sync, write the result to DynamoDB.  
**Main:** That's the real project that we want to overcome timeout limit for it.

![structure](structure.png)

## There is a better way!

If you think structure could be better, please open Issue or PR to share your opinion. We are open to get support!

## Credits

| [<img src="https://avatars3.githubusercontent.com/u/20258973?s=460&u=3147c97360ef8b5d64ef26c77077e1926a686356&v=4" width="100px;"/>](https://github.com/boratanrikulu) <br/>[Bora Tanrıkulu](https://github.com/boratanrikulu)<br/><sub>Developed By</sub><br/> |  
| - |
