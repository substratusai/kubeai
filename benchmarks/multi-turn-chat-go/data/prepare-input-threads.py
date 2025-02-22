import json
import argparse


def main():
    parser = argparse.ArgumentParser(description="A simple program with flags.")
    parser.add_argument(
        "--max-threads", type=int, default=100, help="Max number of threads"
    )
    parser.add_argument(
        "--min-content-length",
        type=int,
        default=1000,
        help="Filter-out threads that contain less than this number of characters",
    )
    parser.add_argument(
        "--max-content-length",
        type=int,
        default=1000000,
        help="Filter-out threads that contain more than this number of characters",
    )
    parser.add_argument(
        "--min-message-count",
        type=int,
        default=3,
        help="Filter-out threads with fewer than this many user messages",
    )
    parser.add_argument(
        "--max-message-count",
        type=int,
        default=1000000,
        help="Filter-out threads with more than this many user messages",
    )
    parser.add_argument(
        "--output",
        type=str,
        default="threads.json",
        help="Output file",
    )
    args = parser.parse_args()
    with open("./raw/ShareGPT_V3_unfiltered_cleaned_split.json", "r") as f:
        data = json.load(f)

    max_threads = args.max_threads

    output = []
    for entry in data:
        conv = entry.get("conversations")
        if conv and conv[0]["from"] == "human" and len(conv[0]["value"]) != 0:
            # Filter the conversation to only include messages from a human using a for loop.
            # entry["userMessages"] = [c["value"] for c in conv if c["from"] == "human"]
            totalContentLength = 0
            messages = []
            for c in conv:
                if c["from"] == "human":
                    content = c["value"]
                    messages.append({"role": "user", "content": content})
                    totalContentLength += len(content)

            if totalContentLength < args.min_content_length:
                continue
            if totalContentLength > args.max_content_length:
                continue

            if len(messages) < args.min_message_count:
                continue
            if len(messages) > args.max_message_count:
                continue

            # Delete the original conversation
            entry["messages"] = messages
            del entry["conversations"]
            output.append(entry)

            if len(output) >= max_threads:
                break

    with open(args.output, "w") as f:
        data = json.dump(output, f, indent=4)


if __name__ == "__main__":
    main()
