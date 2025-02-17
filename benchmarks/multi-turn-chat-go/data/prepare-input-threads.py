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
            messages = []
            for c in conv:
                if c["from"] == "human":
                    content = c["value"]
                    messages.append({"role": "user", "content": content})
                    totalContentLength += len(content)

            if totalContentLength < 2500:
                continue

            if len(messages) < 5:
                continue

            # Delete the original conversation
            entry["messages"] = messages
            del entry["conversations"]
            output.append(entry)

            if len(output) >= max:
                break

    with open("./threads.json", "w") as f:
        data = json.dump(output, f, indent=4)


if __name__ == "__main__":
    main()
