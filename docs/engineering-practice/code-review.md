# Code Review

### What Code Review isn't

* Code Review is not to help you find problems supposed to be found by yourself

  Code Review 不是用来帮你发现你自己能发现的问题的

* Code Review is not responsibility of reviewers. It is code authors' responsibility to help the others review their code.

  Code Review 不是 review 者的义务，帮助他人 review 自己的代码是代码作者的义务

### What Code Review is

* https://en.wikipedia.org/wiki/Code_review
* https://smartbear.com/learn/code-review/best-practices-for-peer-code-review/
* https://coolshell.cn/articles/11432.html

### For Authors

How to help others review your code:

* Review your code by yourself after creating the PR. Fix it if you find a problem.

  **提完 PR 后先自己 review 一遍**，发现问题赶紧偷偷改掉

* Describe the problem the PR solves. Paste related links if there are.

  PR 里说清楚 PR 解决的问题，如有链接则链接之

* Describe how you solved the problem briefly.

  PR 里尽量简要说明 PR 如何解决了对应的问题

* Emphasize the parts which may introduce problems or arguements.

  PR 里指出代码中可能会引入问题或有争议的部分（往往是较复杂或实现较肮脏的部分）

Other tips from [Google Code Review Developer Guide](https://google.github.io/eng-practices/review/)

* Mostly, we expect developers to test CLs well-enough that they work correctly by the time they get to code review.

  提交 Code Review 的 PR 应该是经过自己充分测试，运行 OK 的

* The author of the CL should not include major style changes combined with other changes. It makes it hard to see what is being changed in the CL, makes merges and rollbacks more complex, and causes other problems. For example, if the author wants to reformat the whole file, have them send you just the reformatting as one CL, and then send another CL with their functional changes after that.

  PR 的作者不应该把大的代码风格调整跟别的改动糅在一起。这让真正的改动变得难以发现，也让合并和回滚更加复杂，且会引起其他问题。所以例如写代码的人打算格式化整个文件，把格式化的改动放到一个 PR 里，格式化之后的功能实现放到另一个 PR 里

* 100 lines is usually a reasonable size for a CL, and 1000 lines is usually too large, but it’s up to the judgment of your reviewer.

  100 行改动往往是一个 PR 合理的大小，而 1000 行往往太大，不过具体取决于 review 者的判断

* Sometimes you will encounter situations where it seems like your CL has to be large. This is very rarely true. Authors who practice writing small CLs can almost always find a way to decompose functionality into a series of small changes.

  有时候你会觉得你的 PR 必须得很大，这一般都是扯淡。那些实践了提交小 PR 的习惯的人，总是能找到办法将功能拆解为一系列小的改动

* If a reviewer says that they don’t understand something in your code, your first response should be to clarify the code itself. If the code can’t be clarified, add a code comment that explains why the code is there.

  如果 review 的人说他看不懂你代码里的一些内容，你的第一反应应该是把代码本身组织清楚；如果代码没法被组织清楚，添加注释说明为什么这些代码存在

* However, no matter how certain you are at this point, take a moment to step back and consider if the reviewer is providing valuable feedback that will help the codebase and Google. Your first question to yourself should always be, “Is the reviewer correct?”

  （当收到 comment 时）不管你多么确信自己，花点时间退一步想一下，有可能 review 者正在提供有价值的反馈，帮助整个代码库变得更好。你问自己的第一个问题应该总是：review 者是不是正确的？

* A particular type of complexity is over-engineering, where developers have made the code more generic than it needs to be, or added functionality that isn’t presently needed by the system. Reviewers should be especially vigilant about over-engineering. Encourage developers to solve the problem they know needs to be solved now, not the problem that the developer speculates might need to be solved in the future. The future problem should be solved once it arrives and you can see its actual shape and requirements in the physical universe.

  一个很典型的复杂度来源是过度设计；写代码的人应该解决他们现在需要解决的问题，而不是将来可能需要解决的问题。将来的问题应该在它们到来的时候被解决，那时候你能看到它真切的样子

* The code isn’t more complex than it needs to be.

  代码的复杂度不应该超出它需要的复杂程度

* A CL description is a public record of what change is being made and why it was made. It will become a permanent part of our version control history, and will possibly be read by hundreds of people other than your reviewers over the years.

  一个 PR 的描述应该是一个公开的记录，说明引入了哪些改动，以及为什么引入。这会成为版本控制系统的历史中永久的一部分，且很可能被除 review 者外数以百计的人在以后阅读

* The rest of the description should be informative. It might include a brief description of the problem that’s being solved, and why this is the best approach. If there are any shortcomings to the approach, they should be mentioned. If relevant, include background information such as bug numbers, benchmark results, and links to design documents.

  PR 描述的其他部分应该是提供有用信息的，它可能包括对解决的问题简要的描述，以及为什么当前方案是最好的解决手段。如果当前方案有一些缺陷，也应该提出来。另外，诸如 bug 号、benchmark 结果、设计文档链接等相关背景信息，也应该放进来


### For Reviewers

Tips from [Google Code Review Developer Guide](https://google.github.io/eng-practices/review/):

* The primary purpose of code review is to make sure that the overall code health of Google’s code base is improving over time

  Code Review 最重要的目的是代码整体的质量随时间增长而提升

* In general, reviewers should favor approving a CL once it is in a state where it definitely improves the overall code health of the system being worked on, even if the CL isn’t perfect.

  总的来说，只要 PR 明显可以提升系统整体的代码健康程度，review 者应该倾向 approve 它，即便这个 PR 并不完美

* When coming to consensus becomes especially difficult, it can help to have a face-to-face meeting or a VC between the reviewer and the author, instead of just trying to resolve the conflict through code review comments. (If you do this, though, make sure to record the results of the discussion in a comment on the CL, for future readers.)

  如果 review 者与被 review 者发现较难达成一致，进行一个面对面的交流，而不是尝试继续通过 comment；在达成一致后，把结果记录到讨论的 comment 里

* Ask for unit, integration, or end-to-end tests as appropriate for the change. In general, tests should be added in the same CL as the production code unless the CL is handling an emergency.

  要求 PR 添加改动相关的单元、集成或端到端测试；一般来说，测试代码应该跟对应的改动代码在一个 PR 中添加

* Usually comments are useful when they explain why some code exists, and should not be explaining what some code is doing. If the code isn’t clear enough to explain itself, then the code should be made simpler. There are some exceptions (regular expressions and complex algorithms often benefit greatly from comments that explain what they’re doing, for example) but mostly comments are for information that the code itself can’t possibly contain, like the reasoning behind a decision.

  一般来说，有用的注释是在解释为什么某些代码存在，而不是解释某些代码在做什么。如果代码没有清晰到可以自解释，那么代码应该修改得更简单。有一些例外（比如对于正则或复杂的算法实现，经常需要注释说明它们在做什么）但是绝大多数的注释应该是在说明代码本身无法包含的信息，比如一个决定背后的原因

* Every Line

  每行改动都需要被注意

* If you can’t understand the code, it’s very likely that other developers won’t either. So you’re also helping future developers understand this code, when you ask the developer to clarify it.

  你看不懂的代码，大概率别的开发者也看不懂；所以当你在要求写代码的人把它改得或说明得更清晰的时候，你也是在帮助将来的开发者理解这些代码

* If you follow these guidelines and you are strict with your code reviews, you should find that the entire code review process tends to go faster and faster over time. Developers learn what is required for healthy code, and send you CLs that are great from the start, requiring less and less review time. Reviewers learn to respond quickly and not add unnecessary latency into the review process.

  如果你遵循了这些指导建议，且你对于 Code Review 比较严格，你应该会发现整个 review 的过程会随时间推移变得越来越快。开发者们认识到健康的代码是什么样的，开始给你一些初始质量就比较好的 PR，需要耗费的 review 时间也越来越少。Review 者开始意识到快速响应并不给 review 流程引入不必要的延迟

* If somebody sends you a code review that is so large you’re not sure when you will be able to have time to review it, your typical response should be to ask the developer to split the CL into several smaller CLs that build on each other, instead of one huge CL that has to be reviewed all at once. This is usually possible and very helpful to reviewers, even if it takes additional work from the developer.

  如果有人给你一个很大的 PR，你不确定是不是有时间去 review，你典型的反应应该是要求对方把 PR 拆成几个小的 PR，而不是一个一次性的很大的 PR。这往往是可行的，而且对 review 者很有帮助，即便它会要求写代码的人额外的工作量

Tips from [10 Tips To Write Effective Code Reviews](https://betterprogramming.pub/10-tips-to-write-effective-code-reviews-c25c25aa22c5)

* Always Say Why

  总是说明下为什么（觉得有问题）

* Point Out To Resources

  提供相关资源的链接

* Yours Isn’t Always The Way

  你偏好的做法并不总是唯一（或最好）的做法

Some other tips:

* Use the prefix `Nit: ` to mark unimportant comments, which may simply be discussion or suggestions not meant to block the PR from approving or merging.

  使用 `Nit: ` 前缀标识那些不重要的评论，这些评论可能是讨论性的，也可能是不影响 approve & 合入的改进建议

* Use tools like VSCode GitHub Pull Requests Extension to review big pull requests.

  使用诸如 VSCode 的 [GitHub Pull Requests 插件](https://github.com/microsoft/vscode-pull-request-github) 对较大的 PR 进行 review
