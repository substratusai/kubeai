import json


def main():
    with open("./ShareGPT_V3_unfiltered_cleaned_split.json", "r") as f:
        data = json.load(f)

    # Select a subnet the first conversations that start with a human.
    max = 2000
    output = []
    for entry in data:
        conv = entry.get("conversations")
        if conv and conv[0]["from"] == "human" and len(conv[0]["value"]) != 0:
            # Filter the conversation to only include messages from a human using a for loop.
            # entry["userMessages"] = [c["value"] for c in conv if c["from"] == "human"]
            totalContentLength = 0
            userMessages = []
            for c in conv:
                if c["from"] == "human":
                    content = c["value"]
                    userMessages.append(content)
                    totalContentLength += len(content)

            # Avoid adding conversations that are too long (will exceed context window).
            if totalContentLength > 500:
                continue

            if len(userMessages) < 5:
                continue

            # Delete the original conversation
            entry["userMessages"] = userMessages
            del entry["conversations"]
            output.append(entry)

            if len(output) >= max:
                break

    with open("./message-threads.json", "w") as f:
        data = json.dump(output, f, indent=4)


if __name__ == "__main__":
    main()
