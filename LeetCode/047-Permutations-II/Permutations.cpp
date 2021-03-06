/*
 * Given a collection of numbers that might contain duplicates, return all possible unique permutations.
 *
 * For example,
 * [1,1,2] have the following unique permutations:
 * [1,1,2], [1,2,1], and [2,1,1].
 */

#include <iostream>
#include <vector>

using namespace std;

class Solution {
public:

    vector<vector<int> > result;
    vector<bool> visited;

    void DFS(vector<int> &nums,vector<int> &current,vector<bool> &visited){
        if(current.size()==nums.size()){
            result.push_back(current);
            return;
        }
        if(current.size()>nums.size()){
            return;
        }
        for(unsigned int i=0;i<nums.size();i++){
            if(visited[i]==true){
                continue;
            }
            bool flag=false;
            for(unsigned int j=0;j<i;j++){
                if(nums[j]==nums[i]&&visited[j]==false){
                    flag=true;
                    break;
                }
            }
            if(flag){
                continue;
            }
            visited[i]=true;
            current.push_back(nums[i]);
            DFS(nums,current,visited);
            current.pop_back();
            visited[i]=false;
        }
        return;
    }

    vector<vector<int> > permuteUnique(vector<int>& nums) {
        for(unsigned int i=0;i<nums.size();i++){
            visited.push_back(false);
        }
        result.clear();
        vector<int> current;
        DFS(nums,current,visited);
        return result;
    }
};

int main()
{
    vector<int> nums;
    nums.push_back(1);
    nums.push_back(2);
    nums.push_back(2);
    Solution object;
    vector<vector<int> > result=object.permuteUnique(nums);
    for(unsigned int i=0;i<result.size();i++){
        for(unsigned int j=0;j<result[i].size();j++){
            cout<<result[i][j]<<" ";
        }
        cout<<endl;
    }
    return 0;
}
