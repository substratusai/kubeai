#!/usr/bin/env python
import argparse
import yaml
import sys


def traverse(obj, components, collected):
    """
    Recursively search an object (dict or list) for internal $ref keys that
    point to components (i.e. starting with "#/components/"). For each referenced
    component, add it to the collected set and traverse its definition to find
    further references.
    """
    if isinstance(obj, dict):
        if "$ref" in obj and isinstance(obj["$ref"], str):
            ref = obj["$ref"]
            if ref.startswith("#/components/"):
                # Expected format: "#/components/<component_type>/<component_name>"
                parts = ref.split("/")
                if len(parts) >= 4:
                    comp_type = parts[2]
                    comp_name = parts[3]
                    if (comp_type, comp_name) not in collected:
                        collected.add((comp_type, comp_name))
                        try:
                            comp_def = components[comp_type][comp_name]
                            traverse(comp_def, components, collected)
                        except KeyError:
                            # If the referenced component isn't found, skip it.
                            pass
        for value in obj.values():
            traverse(value, components, collected)
    elif isinstance(obj, list):
        for item in obj:
            traverse(item, components, collected)


def main():
    parser = argparse.ArgumentParser(
        description="Filter an OpenAPI spec down to a specific path and method, including only the relevant components."
    )
    parser.add_argument(
        "spec_file", help="Path to the OpenAPI spec file (YAML or JSON)"
    )
    parser.add_argument("path", help="Path to filter on (e.g. '/chat/completions')")
    parser.add_argument("method", help="HTTP method (e.g. 'post')")
    parser.add_argument(
        "--output",
        "-o",
        help="Output file; if not provided, prints to stdout",
        default=None,
    )
    args = parser.parse_args()

    # Load the OpenAPI spec
    try:
        with open(args.spec_file, "r") as f:
            spec = yaml.safe_load(f)
    except Exception as e:
        print(f"Error loading spec file: {e}")
        sys.exit(1)

    # Validate the existence of the requested path
    if "paths" not in spec:
        print("The spec does not contain any paths.")
        sys.exit(1)
    if args.path not in spec["paths"]:
        print(f"Path '{args.path}' not found in the spec.")
        sys.exit(1)

    path_item = spec["paths"][args.path]
    method_lower = args.method.lower()
    if method_lower not in path_item:
        print(f"Method '{args.method}' not found for path '{args.path}'.")
        sys.exit(1)
    operation = path_item[method_lower]

    # Get the components section from the spec (if any)
    components = spec.get("components", {})

    # Traverse the operation definition to collect all component $refs
    collected = set()
    traverse(operation, components, collected)

    # Build a new components dictionary with only the collected components.
    new_components = {}
    for comp_type, comp_name in collected:
        if comp_type not in new_components:
            new_components[comp_type] = {}
        # It is assumed that the component exists, since we gathered it during traversal.
        new_components[comp_type][comp_name] = components[comp_type][comp_name]

    # Build the new paths dict with only the requested operation.
    new_paths = {args.path: {method_lower: operation}}

    # Construct the new spec. We preserve key top-level elements such as openapi, info, and servers.
    new_spec = {}
    for key in [
        "openapi",
        "info",
        "servers",
        "paths",
        "components",
        "tags",
        "externalDocs",
    ]:
        if key == "paths":
            new_spec[key] = new_paths
        elif key == "components":
            new_spec[key] = new_components
        elif key in spec:
            new_spec[key] = spec[key]

    # Output the filtered spec as YAML.
    output_yaml = yaml.dump(new_spec, sort_keys=False)
    if args.output:
        try:
            with open(args.output, "w") as out:
                out.write(output_yaml)
            print(f"Filtered spec written to {args.output}")
        except Exception as e:
            print(f"Error writing output file: {e}")
    else:
        print(output_yaml)


if __name__ == "__main__":
    main()
