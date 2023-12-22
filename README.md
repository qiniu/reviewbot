## staticcheck webhook server

### Get Started


### TODO:
* 支持选择是在PR文件里Comments，还是直接在对话里Comments
* 支持是选择只反馈变动的diff，还是diff所在的文件(当前默认是所在的文件)
    * 类似 codecov 的 check-run-annotations
* 做成staticaction,能comment PR
* 做成Github APP
* 做个metric页面，实时展示发现了多少问题



#### 

* 1. Get PR infos:

Example:
``` json
[
  {
    "sha": "bbcd538c8e72b8c175046e27cc8f907076331401",
    "filename": "file1.txt",
    "status": "added",
    "additions": 103,
    "deletions": 21,
    "changes": 124,
    "blob_url": "https://github.com/octocat/Hello-World/blob/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt",
    "raw_url": "https://github.com/octocat/Hello-World/raw/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt",
    "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
    "patch": "@@ -132,7 +132,7 @@ module Test @@ -1000,7 +1000,7 @@ module Test"
  }
]
```
schema:

```json
{
  "type": "array",
  "items": {
    "title": "Diff Entry",
    "description": "Diff Entry",
    "type": "object",
    "properties": {
      "sha": {
        "type": "string",
        "examples": [
          "bbcd538c8e72b8c175046e27cc8f907076331401"
        ]
      },
      "filename": {
        "type": "string",
        "examples": [
          "file1.txt"
        ]
      },
      "status": {
        "type": "string",
        "enum": [
          "added",
          "removed",
          "modified",
          "renamed",
          "copied",
          "changed",
          "unchanged"
        ],
        "examples": [
          "added"
        ]
      },
      "additions": {
        "type": "integer",
        "examples": [
          103
        ]
      },
      "deletions": {
        "type": "integer",
        "examples": [
          21
        ]
      },
      "changes": {
        "type": "integer",
        "examples": [
          124
        ]
      },
      "blob_url": {
        "type": "string",
        "format": "uri",
        "examples": [
          "https://github.com/octocat/Hello-World/blob/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt"
        ]
      },
      "raw_url": {
        "type": "string",
        "format": "uri",
        "examples": [
          "https://github.com/octocat/Hello-World/raw/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt"
        ]
      },
      "contents_url": {
        "type": "string",
        "format": "uri",
        "examples": [
          "https://api.github.com/repos/octocat/Hello-World/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e"
        ]
      },
      "patch": {
        "type": "string",
        "examples": [
          "@@ -132,7 +132,7 @@ module Test @@ -1000,7 +1000,7 @@ module Test"
        ]
      },
      "previous_filename": {
        "type": "string",
        "examples": [
          "file.txt"
        ]
      }
    },
    "required": [
      "additions",
      "blob_url",
      "changes",
      "contents_url",
      "deletions",
      "filename",
      "raw_url",
      "sha",
      "status"
    ]
  }
}
```


* 2. List Review Comment

Example:
```json
[
  {
    "url": "https://api.github.com/repos/octocat/Hello-World/pulls/comments/1",
    "pull_request_review_id": 42,
    "id": 10,
    "node_id": "MDI0OlB1bGxSZXF1ZXN0UmV2aWV3Q29tbWVudDEw",
    "diff_hunk": "@@ -16,33 +16,40 @@ public class Connection : IConnection...",
    "path": "file1.txt",
    "position": 1,
    "original_position": 4,
    "commit_id": "6dcb09b5b57875f334f61aebed695e2e4193db5e",
    "original_commit_id": "9c48853fa3dc5c1c3d6f1f1cd1f2743e72652840",
    "in_reply_to_id": 8,
    "user": {
      "login": "octocat",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/octocat_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/octocat",
      "html_url": "https://github.com/octocat",
      "followers_url": "https://api.github.com/users/octocat/followers",
      "following_url": "https://api.github.com/users/octocat/following{/other_user}",
      "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
      "organizations_url": "https://api.github.com/users/octocat/orgs",
      "repos_url": "https://api.github.com/users/octocat/repos",
      "events_url": "https://api.github.com/users/octocat/events{/privacy}",
      "received_events_url": "https://api.github.com/users/octocat/received_events",
      "type": "User",
      "site_admin": false
    },
    "body": "Great stuff!",
    "created_at": "2011-04-14T16:00:49Z",
    "updated_at": "2011-04-14T16:00:49Z",
    "html_url": "https://github.com/octocat/Hello-World/pull/1#discussion-diff-1",
    "pull_request_url": "https://api.github.com/repos/octocat/Hello-World/pulls/1",
    "author_association": "NONE",
    "_links": {
      "self": {
        "href": "https://api.github.com/repos/octocat/Hello-World/pulls/comments/1"
      },
      "html": {
        "href": "https://github.com/octocat/Hello-World/pull/1#discussion-diff-1"
      },
      "pull_request": {
        "href": "https://api.github.com/repos/octocat/Hello-World/pulls/1"
      }
    },
    "start_line": 1,
    "original_start_line": 1,
    "start_side": "RIGHT",
    "line": 2,
    "original_line": 2,
    "side": "RIGHT"
  }
]

```
schema:
```json
{
  "type": "array",
  "items": {
    "title": "Pull Request Review Comment",
    "description": "Pull Request Review Comments are comments on a portion of the Pull Request's diff.",
    "type": "object",
    "properties": {
      "url": {
        "description": "URL for the pull request review comment",
        "type": "string",
        "examples": [
          "https://api.github.com/repos/octocat/Hello-World/pulls/comments/1"
        ]
      },
      "pull_request_review_id": {
        "description": "The ID of the pull request review to which the comment belongs.",
        "type": [
          "integer",
          "null"
        ],
        "examples": [
          42
        ]
      },
      "id": {
        "description": "The ID of the pull request review comment.",
        "type": "integer",
        "examples": [
          1
        ]
      },
      "node_id": {
        "description": "The node ID of the pull request review comment.",
        "type": "string",
        "examples": [
          "MDI0OlB1bGxSZXF1ZXN0UmV2aWV3Q29tbWVudDEw"
        ]
      },
      "diff_hunk": {
        "description": "The diff of the line that the comment refers to.",
        "type": "string",
        "examples": [
          "@@ -16,33 +16,40 @@ public class Connection : IConnection..."
        ]
      },
      "path": {
        "description": "The relative path of the file to which the comment applies.",
        "type": "string",
        "examples": [
          "config/database.yaml"
        ]
      },
      "position": {
        "description": "The line index in the diff to which the comment applies. This field is deprecated; use `line` instead.",
        "type": "integer",
        "examples": [
          1
        ]
      },
      "original_position": {
        "description": "The index of the original line in the diff to which the comment applies. This field is deprecated; use `original_line` instead.",
        "type": "integer",
        "examples": [
          4
        ]
      },
      "commit_id": {
        "description": "The SHA of the commit to which the comment applies.",
        "type": "string",
        "examples": [
          "6dcb09b5b57875f334f61aebed695e2e4193db5e"
        ]
      },
      "original_commit_id": {
        "description": "The SHA of the original commit to which the comment applies.",
        "type": "string",
        "examples": [
          "9c48853fa3dc5c1c3d6f1f1cd1f2743e72652840"
        ]
      },
      "in_reply_to_id": {
        "description": "The comment ID to reply to.",
        "type": "integer",
        "examples": [
          8
        ]
      },
      "user": {
        "title": "Simple User",
        "description": "A GitHub user.",
        "type": "object",
        "properties": {
          "name": {
            "type": [
              "string",
              "null"
            ]
          },
          "email": {
            "type": [
              "string",
              "null"
            ]
          },
          "login": {
            "type": "string",
            "examples": [
              "octocat"
            ]
          },
          "id": {
            "type": "integer",
            "examples": [
              1
            ]
          },
          "node_id": {
            "type": "string",
            "examples": [
              "MDQ6VXNlcjE="
            ]
          },
          "avatar_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://github.com/images/error/octocat_happy.gif"
            ]
          },
          "gravatar_id": {
            "type": [
              "string",
              "null"
            ],
            "examples": [
              "41d064eb2195891e12d0413f63227ea7"
            ]
          },
          "url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://api.github.com/users/octocat"
            ]
          },
          "html_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://github.com/octocat"
            ]
          },
          "followers_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://api.github.com/users/octocat/followers"
            ]
          },
          "following_url": {
            "type": "string",
            "examples": [
              "https://api.github.com/users/octocat/following{/other_user}"
            ]
          },
          "gists_url": {
            "type": "string",
            "examples": [
              "https://api.github.com/users/octocat/gists{/gist_id}"
            ]
          },
          "starred_url": {
            "type": "string",
            "examples": [
              "https://api.github.com/users/octocat/starred{/owner}{/repo}"
            ]
          },
          "subscriptions_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://api.github.com/users/octocat/subscriptions"
            ]
          },
          "organizations_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://api.github.com/users/octocat/orgs"
            ]
          },
          "repos_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://api.github.com/users/octocat/repos"
            ]
          },
          "events_url": {
            "type": "string",
            "examples": [
              "https://api.github.com/users/octocat/events{/privacy}"
            ]
          },
          "received_events_url": {
            "type": "string",
            "format": "uri",
            "examples": [
              "https://api.github.com/users/octocat/received_events"
            ]
          },
          "type": {
            "type": "string",
            "examples": [
              "User"
            ]
          },
          "site_admin": {
            "type": "boolean"
          },
          "starred_at": {
            "type": "string",
            "examples": [
              "\"2020-07-09T00:17:55Z\""
            ]
          }
        },
        "required": [
          "avatar_url",
          "events_url",
          "followers_url",
          "following_url",
          "gists_url",
          "gravatar_id",
          "html_url",
          "id",
          "node_id",
          "login",
          "organizations_url",
          "received_events_url",
          "repos_url",
          "site_admin",
          "starred_url",
          "subscriptions_url",
          "type",
          "url"
        ]
      },
      "body": {
        "description": "The text of the comment.",
        "type": "string",
        "examples": [
          "We should probably include a check for null values here."
        ]
      },
      "created_at": {
        "type": "string",
        "format": "date-time",
        "examples": [
          "2011-04-14T16:00:49Z"
        ]
      },
      "updated_at": {
        "type": "string",
        "format": "date-time",
        "examples": [
          "2011-04-14T16:00:49Z"
        ]
      },
      "html_url": {
        "description": "HTML URL for the pull request review comment.",
        "type": "string",
        "format": "uri",
        "examples": [
          "https://github.com/octocat/Hello-World/pull/1#discussion-diff-1"
        ]
      },
      "pull_request_url": {
        "description": "URL for the pull request that the review comment belongs to.",
        "type": "string",
        "format": "uri",
        "examples": [
          "https://api.github.com/repos/octocat/Hello-World/pulls/1"
        ]
      },
      "author_association": {
        "title": "author_association",
        "type": "string",
        "description": "How the author is associated with the repository.",
        "enum": [
          "COLLABORATOR",
          "CONTRIBUTOR",
          "FIRST_TIMER",
          "FIRST_TIME_CONTRIBUTOR",
          "MANNEQUIN",
          "MEMBER",
          "NONE",
          "OWNER"
        ],
        "examples": [
          "OWNER"
        ]
      },
      "_links": {
        "type": "object",
        "properties": {
          "self": {
            "type": "object",
            "properties": {
              "href": {
                "type": "string",
                "format": "uri",
                "examples": [
                  "https://api.github.com/repos/octocat/Hello-World/pulls/comments/1"
                ]
              }
            },
            "required": [
              "href"
            ]
          },
          "html": {
            "type": "object",
            "properties": {
              "href": {
                "type": "string",
                "format": "uri",
                "examples": [
                  "https://github.com/octocat/Hello-World/pull/1#discussion-diff-1"
                ]
              }
            },
            "required": [
              "href"
            ]
          },
          "pull_request": {
            "type": "object",
            "properties": {
              "href": {
                "type": "string",
                "format": "uri",
                "examples": [
                  "https://api.github.com/repos/octocat/Hello-World/pulls/1"
                ]
              }
            },
            "required": [
              "href"
            ]
          }
        },
        "required": [
          "self",
          "html",
          "pull_request"
        ]
      },
      "start_line": {
        "type": [
          "integer",
          "null"
        ],
        "description": "The first line of the range for a multi-line comment.",
        "examples": [
          2
        ]
      },
      "original_start_line": {
        "type": [
          "integer",
          "null"
        ],
        "description": "The first line of the range for a multi-line comment.",
        "examples": [
          2
        ]
      },
      "start_side": {
        "type": [
          "string",
          "null"
        ],
        "description": "The side of the first line of the range for a multi-line comment.",
        "enum": [
          "LEFT",
          "RIGHT",
          null
        ],
        "default": "RIGHT"
      },
      "line": {
        "description": "The line of the blob to which the comment applies. The last line of the range for a multi-line comment",
        "type": "integer",
        "examples": [
          2
        ]
      },
      "original_line": {
        "description": "The line of the blob to which the comment applies. The last line of the range for a multi-line comment",
        "type": "integer",
        "examples": [
          2
        ]
      },
      "side": {
        "description": "The side of the diff to which the comment applies. The side of the last line of the range for a multi-line comment",
        "enum": [
          "LEFT",
          "RIGHT"
        ],
        "default": "RIGHT",
        "type": "string"
      },
      "subject_type": {
        "description": "The level at which the comment is targeted, can be a diff line or a file.",
        "type": "string",
        "enum": [
          "line",
          "file"
        ]
      },
      "reactions": {
        "title": "Reaction Rollup",
        "type": "object",
        "properties": {
          "url": {
            "type": "string",
            "format": "uri"
          },
          "total_count": {
            "type": "integer"
          },
          "+1": {
            "type": "integer"
          },
          "-1": {
            "type": "integer"
          },
          "laugh": {
            "type": "integer"
          },
          "confused": {
            "type": "integer"
          },
          "heart": {
            "type": "integer"
          },
          "hooray": {
            "type": "integer"
          },
          "eyes": {
            "type": "integer"
          },
          "rocket": {
            "type": "integer"
          }
        },
        "required": [
          "url",
          "total_count",
          "+1",
          "-1",
          "laugh",
          "confused",
          "heart",
          "hooray",
          "eyes",
          "rocket"
        ]
      },
      "body_html": {
        "type": "string",
        "examples": [
          "\"<p>comment body</p>\""
        ]
      },
      "body_text": {
        "type": "string",
        "examples": [
          "\"comment body\""
        ]
      }
    },
    "required": [
      "url",
      "id",
      "node_id",
      "pull_request_review_id",
      "diff_hunk",
      "path",
      "commit_id",
      "original_commit_id",
      "user",
      "body",
      "created_at",
      "updated_at",
      "html_url",
      "pull_request_url",
      "author_association",
      "_links"
    ]
  }
}
```

