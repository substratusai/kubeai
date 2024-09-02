# Documentation

We are grateful for anyone who takes the time to improve KubeAI documentation! In order to keep our docs clear and consistent we ask that you first read about the approach to documentation that we have standardized on...

## Read before writing!

The KubeAI approach to documentation is loosely inspired by the [Diataxis](https://diataxis.fr/) method.

TLDR on how KubeAI docs are organized:

* **Installation**: How-to guides specific to installing KubeAI.
* **How To**: Directions that guide the reader through a problem or towards a result. How-to guides are goal-oriented. They assume the user is familiar with general concepts, tools, and has already installed KubeAI.
* **Concepts**: A reflective explanation of KubeAI topics with a focus on giving the reader an understanding of the why.
* **Tutorials**: Learning oriented experiences. Lessons that often guide a user from beginning to end. The goal is to help the reader *learn* something (compared to a how-to guide that is focused on helping the reader *do* something).
* **Contributing**: The docs in here differ from the rest of the docs by audience: these docs are for anyone who will be contributing code or docs to the KubeAI project.

## How to serve kubeai.org locally

Make sure you have python3 installed and run:

```bash
make docs
```