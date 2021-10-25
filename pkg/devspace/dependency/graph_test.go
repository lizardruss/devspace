package dependency

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestGraph(t *testing.T) {
	var (
		root                   = newNode("root", nil)
		rootChild1             = newNode("rootChild1", nil)
		rootChild2             = newNode("rootChild2", nil)
		rootChild3             = newNode("rootChild3", nil)
		rootChild2Child1       = newNode("rootChild2Child1", nil)
		rootChild2Child1Child1 = newNode("rootChild2Child1Child1", nil)

		testGraph = newGraph(root)
	)

	_, err := testGraph.insertNodeAt("does not exits", rootChild1.ID, nil)
	if err == nil {
		t.Fatal("insertNodeAt error expected")
	}

	_, _ = testGraph.insertNodeAt(root.ID, rootChild1.ID, nil)
	_, _ = testGraph.insertNodeAt(root.ID, rootChild2.ID, nil)
	_, _ = testGraph.insertNodeAt(root.ID, rootChild3.ID, nil)

	_, _ = testGraph.insertNodeAt(rootChild2.ID, rootChild2Child1.ID, nil)
	_, _ = testGraph.insertNodeAt(rootChild2Child1.ID, rootChild2Child1Child1.ID, nil)
	_, _ = testGraph.insertNodeAt(rootChild3.ID, rootChild2.ID, nil)

	// Cyclic graph error
	_, err = testGraph.insertNodeAt(rootChild2Child1Child1.ID, rootChild3.ID, nil)
	if err == nil {
		t.Fatal("Cyclic error expected")
	} else {
		errMsg := `Cyclic dependency found: 
rootChild2Child1Child1
rootChild3
rootChild2
rootChild2Child1
rootChild2Child1Child1`

		if err.Error() != errMsg {
			t.Fatalf("Expected %s, got %s", errMsg, err.Error())
		}
	}

	// Find first path
	path := findFirstPath(rootChild1, rootChild2)
	if path != nil {
		t.Fatalf("Wrong path found: %#+v", path)
	}

	// Find first path
	path = findFirstPath(root, rootChild2Child1Child1)
	if len(path) != 4 || path[0].ID != root.ID || path[1].ID != rootChild2.ID || path[2].ID != rootChild2Child1.ID || path[3].ID != rootChild2Child1Child1.ID {
		t.Fatalf("Wrong path found: %#+v", path)
	}

	// preOrder traversal
	expected := []string{"root", "rootChild1", "rootChild2", "rootChild2Child1", "rootChild2Child1Child1", "rootChild3", "rootChild2", "rootChild2Child1", "rootChild2Child1Child1"}
	actual := []string{}
	testGraph.preOrderSearch(testGraph.Root, func(n *node) (bool, error) {
		actual = append(actual, n.ID)
		return false, nil
	})
	assert.DeepEqual(t, expected, actual)

	// postOrder traversal
	expected = []string{"rootChild1", "rootChild2Child1Child1", "rootChild2Child1", "rootChild2", "rootChild2Child1Child1", "rootChild2Child1", "rootChild2", "rootChild3", "root"}
	actual = []string{}
	testGraph.postOrderSearch(testGraph.Root, func(n *node) (bool, error) {
		actual = append(actual, n.ID)
		return false, nil
	})
	fmt.Println(actual)
	assert.DeepEqual(t, expected, actual)

	// Get leaf node
	leaf := testGraph.getNextLeaf(root)
	if leaf.ID != rootChild1.ID {
		t.Fatalf("GetLeaf1: Got id %s, expected %s", leaf.ID, rootChild1.ID)
	}

	err = testGraph.addEdge("NotThere", leaf.ID)
	if err == nil {
		t.Fatal("No error when adding an edge from a non-existing node")
	}

	err = testGraph.addEdge(leaf.ID, "NotThere")
	if err == nil {
		t.Fatal("No error when adding an edge to a non-existing node")
	}

	// preOrder Seach for 5th item
	count := 0
	found, _ := testGraph.preOrderSearch(testGraph.Root, func(n *node) (bool, error) {
		result := count == 5
		count++
		return result, nil
	})
	assert.Equal(t, found.ID, "rootChild3")

	// postOrder Search
	count = 0
	found, _ = testGraph.postOrderSearch(testGraph.Root, func(n *node) (bool, error) {
		result := count == 5
		count++
		return result, nil
	})
	assert.Equal(t, found.ID, "rootChild2Child1")

	// Prune tree until empty
	expected = []string{"rootChild1", "rootChild2Child1Child1", "rootChild2Child1", "rootChild2", "rootChild3"}
	actual = []string{}

	for testGraph.len() > 0 {
		curr := testGraph.getNextLeaf(testGraph.Root)
		actual = append(actual, curr.ID)

		err := testGraph.removeNode(curr.ID)
		if err != nil {
			t.Fatal(err)
		}
	}

	fmt.Println(actual)
	assert.DeepEqual(t, expected, actual)
}
